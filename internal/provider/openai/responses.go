package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/ua"
)

// responsesRequest represents the request body for OpenAI Responses API.
type responsesRequest struct {
	Model                string               `json:"model"`
	Instructions         string               `json:"instructions,omitempty"`
	Input                []responsesInputItem `json:"input"`
	Tools                []responsesTool      `json:"tools,omitempty"`
	MaxOutputTokens      int                  `json:"max_output_tokens,omitempty"`
	Temperature          *float64             `json:"temperature,omitempty"`
	TopP                 *float64             `json:"top_p,omitempty"`
	Stream               bool                 `json:"stream"`
	Reasoning            *responsesReasoning  `json:"reasoning,omitempty"`
	ParallelToolCalls    *bool                `json:"parallel_tool_calls,omitempty"`
	PromptCacheKey       string               `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string               `json:"prompt_cache_retention,omitempty"`
}

type responsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type responsesInputItem struct {
	Type      string      `json:"type,omitempty"`
	Role      string      `json:"role,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	CallID    string      `json:"call_id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Arguments string      `json:"arguments,omitempty"`
	Output    string      `json:"output,omitempty"`
}

type responsesContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type responsesSSEEvent struct {
	Type        string                    `json:"type"`
	Delta       string                    `json:"delta,omitempty"`
	ItemID      string                    `json:"item_id,omitempty"`
	OutputIndex int                       `json:"output_index,omitempty"`
	Item        *responsesOutputItem      `json:"item,omitempty"`
	Response    *responsesCompletedObject `json:"response,omitempty"`
	Error       *responsesError           `json:"error,omitempty"`
}

type responsesOutputItem struct {
	ID        string `json:"id,omitempty"`
	Type      string `json:"type,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type responsesCompletedObject struct {
	Status string          `json:"status,omitempty"`
	Usage  *responsesUsage `json:"usage,omitempty"`
	Error  *responsesError `json:"error,omitempty"`
}

type responsesError struct {
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	Type    string `json:"type,omitempty"`
}

type responsesUsage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	TotalTokens        int `json:"total_tokens"`
	InputTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details,omitempty"`
}

func (p *Provider) chatResponses(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
	ch := make(chan provider.StreamEvent, 100)

	go func() {
		defer close(ch)

		if p.apiKey == "" {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("OPENAI_API_KEY not set")}
			return
		}

		modelID := params.ModelID
		if modelID == "" {
			if len(p.Models()) > 0 {
				modelID = p.Models()[0].ID
			} else {
				modelID = "gpt-4o"
			}
		}

		maxTokens := params.MaxTokens
		if maxTokens == 0 {
			maxTokens = 16384
		}
		model := p.GetModel(modelID)

		reqBody := responsesRequest{
			Model:           modelID,
			Instructions:    params.SystemPrompt,
			Input:           p.convertResponsesInput(params),
			Tools:           p.convertResponsesTools(params.Tools),
			MaxOutputTokens: maxTokens,
			Temperature:     params.Temperature,
			TopP:            params.TopP,
			Stream:          true,
		}

		if p.responsesConfig != nil && p.responsesConfig.promptCacheEnabled && supportsPromptCacheKey(model) {
			reqBody.PromptCacheKey = p.responsesPromptCacheKey(modelID)
			if supportsPromptCacheRetention(model) {
				reqBody.PromptCacheRetention = p.responsesConfig.promptCacheRetention
			}
		}

		if !p.disableReasoning && params.ThinkingLevel != provider.ThinkingOff && model != nil && model.Reasoning {
			reqBody.Reasoning = &responsesReasoning{
				Effort:  responsesReasoningEffort(params.ThinkingLevel),
				Summary: p.responsesReasoningSummary(model),
			}
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("marshal request: %w", err)}
			return
		}
		if os.Getenv("VIBECODING_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] Responses request body: %s\n", string(body))
		}

		maxRetries := 0
		baseDelayMs := 2000
		if p.retryConfig != nil && p.retryConfig.Enabled {
			maxRetries = p.retryConfig.MaxRetries
			baseDelayMs = p.retryConfig.BaseDelayMs
		}

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if err := ctx.Err(); err != nil {
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: err, StopReason: "aborted"}
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/responses", bytes.NewReader(body))
			if err != nil {
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("create request: %w", err)}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("User-Agent", ua.ProviderUserAgent())

			resp, err := p.client.Do(req)
			if err != nil {
				if attempt < maxRetries && provider.IsRetryable(err, 0) {
					delay := provider.RetryDelay(attempt, baseDelayMs)
					ch <- provider.StreamEvent{Type: provider.StreamRetry, RetryAttempt: attempt + 1, RetryMax: maxRetries, Error: fmt.Errorf("%s", provider.FormatRetryMessage(attempt, maxRetries, delay, err))}
					select {
					case <-ctx.Done():
						ch <- provider.StreamEvent{Type: provider.StreamError, Error: ctx.Err(), StopReason: "aborted"}
						return
					case <-time.After(delay):
					}
					continue
				}
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("send request: %w", err)}
				return
			}

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if attempt < maxRetries && provider.IsRetryable(nil, resp.StatusCode) {
					delay := provider.RetryDelay(attempt, baseDelayMs)
					ch <- provider.StreamEvent{Type: provider.StreamRetry, RetryAttempt: attempt + 1, RetryMax: maxRetries, Error: fmt.Errorf("%s", provider.FormatRetryMessage(attempt, maxRetries, delay, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))))}
					select {
					case <-ctx.Done():
						ch <- provider.StreamEvent{Type: provider.StreamError, Error: ctx.Err(), StopReason: "aborted"}
						return
					case <-time.After(delay):
					}
					continue
				}
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))}
				return
			}

			p.parseResponsesSSE(ctx, resp.Body, ch, params)
			resp.Body.Close()
			return
		}

		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("all %d retry attempts exhausted", maxRetries)}
	}()

	return ch
}

func (p *Provider) convertResponsesInput(params provider.ChatParams) []responsesInputItem {
	items := make([]responsesInputItem, 0, len(params.Messages))
	for _, msg := range params.Messages {
		switch msg.Role {
		case "toolResult":
			items = append(items, responsesInputItem{Type: "function_call_output", CallID: msg.ToolCallID, Output: responseToolOutput(msg)})
		case "assistant":
			content := p.responsesMessageContent(msg, "output_text")
			if content != nil {
				items = append(items, responsesInputItem{Type: "message", Role: "assistant", Content: content})
			}
			for _, c := range msg.Contents {
				if c.Type == "toolCall" && c.ToolCall != nil {
					items = append(items, responsesInputItem{Type: "function_call", CallID: c.ToolCall.ID, Name: c.ToolCall.Name, Arguments: string(c.ToolCall.Arguments)})
				}
			}
		default:
			role := msg.Role
			if role == "" {
				role = "user"
			}
			content := p.responsesMessageContent(msg, "input_text")
			items = append(items, responsesInputItem{Type: "message", Role: role, Content: content})
		}
	}
	return items
}

func (p *Provider) responsesMessageContent(msg provider.Message, textType string) interface{} {
	if len(msg.Contents) == 0 {
		return []responsesContentBlock{{Type: textType, Text: msg.Content}}
	}
	blocks := make([]responsesContentBlock, 0, len(msg.Contents))
	for _, c := range msg.Contents {
		switch c.Type {
		case "text":
			blocks = append(blocks, responsesContentBlock{Type: textType, Text: c.Text})
		case "image":
			if c.Image != nil {
				blocks = append(blocks, responsesContentBlock{Type: "input_image", ImageURL: fmt.Sprintf("data:%s;base64,%s", c.Image.MimeType, c.Image.Data)})
			}
		}
	}
	if len(blocks) == 0 && msg.Content != "" {
		blocks = append(blocks, responsesContentBlock{Type: textType, Text: msg.Content})
	}
	return blocks
}

func responseToolOutput(msg provider.Message) string {
	if msg.Content != "" || len(msg.Contents) == 0 {
		return msg.Content
	}
	var parts []string
	for _, c := range msg.Contents {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func (p *Provider) convertResponsesTools(tools []provider.ToolDefinition) []responsesTool {
	result := make([]responsesTool, 0, len(tools))
	for _, t := range tools {
		if t.Kind == "hosted" {
			toolType := hostedResponsesToolType(t)
			if toolType == "" {
				continue
			}
			result = append(result, responsesTool{Type: toolType})
			continue
		}
		result = append(result, responsesTool{Type: "function", Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	return result
}

func hostedResponsesToolType(t provider.ToolDefinition) string {
	switch {
	case t.ProviderType == "responses" && t.Name == "web_search":
		return "web_search"
	default:
		return ""
	}
}

func (p *Provider) parseResponsesSSE(ctx context.Context, body io.Reader, ch chan<- provider.StreamEvent, params provider.ChatParams) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		usage           *provider.Usage
		stopReason      string
		toolCallsByKey  = make(map[string]*provider.ToolCallBlock)
		toolCallOrder   []string
		argumentBuffers = make(map[string]*strings.Builder)
	)

	ch <- provider.StreamEvent{Type: provider.StreamStart}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: ctx.Err(), StopReason: "aborted"}
			return
		case <-params.Abort:
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("aborted"), StopReason: "aborted"}
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event responsesSSEEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" {
				ch <- provider.StreamEvent{Type: provider.StreamTextDelta, TextDelta: event.Delta}
			}
		case "response.reasoning_text.delta":
			if !p.disableReasoning && event.Delta != "" {
				ch <- provider.StreamEvent{Type: provider.StreamThinkDelta, ThinkDelta: event.Delta}
			}
		case "response.function_call_arguments.delta":
			key := responsesToolKey(event.ItemID, event.OutputIndex)
			if _, ok := argumentBuffers[key]; !ok {
				argumentBuffers[key] = &strings.Builder{}
			}
			argumentBuffers[key].WriteString(event.Delta)
		case "response.output_item.done":
			if event.Item != nil && event.Item.Type == "function_call" {
				key := responsesToolKey(event.Item.ID, event.OutputIndex)
				tc := &provider.ToolCallBlock{ID: event.Item.CallID, Name: event.Item.Name, Arguments: json.RawMessage(event.Item.Arguments)}
				if tc.ID == "" {
					tc.ID = event.Item.ID
				}
				if tc.ID == "" {
					tc.ID = "toolcall_" + strconv.Itoa(len(toolCallOrder))
				}
				if tc.Arguments == nil || len(tc.Arguments) == 0 {
					if buf := argumentBuffers[key]; buf != nil {
						tc.Arguments = json.RawMessage(buf.String())
					}
				}
				if _, seen := toolCallsByKey[key]; !seen {
					toolCallOrder = append(toolCallOrder, key)
				}
				toolCallsByKey[key] = tc
			}
		case "response.completed":
			if event.Response != nil {
				usage = convertResponsesUsage(event.Response.Usage)
				stopReason = responseStopReason(event.Response.Status)
				if event.Response.Error != nil {
					ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("responses error: %s", event.Response.Error.Message), StopReason: "error"}
					return
				}
			}
		case "response.failed", "error":
			if event.Error != nil {
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("responses error: %s", event.Error.Message), StopReason: "error"}
				return
			}
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("responses stream failed"), StopReason: "error"}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("stream read error: %w", err), StopReason: "error"}
		return
	}

	for _, key := range toolCallOrder {
		if tc := toolCallsByKey[key]; tc != nil {
			ch <- provider.StreamEvent{Type: provider.StreamToolCall, ToolCall: tc}
		}
	}
	if usage != nil {
		ch <- provider.StreamEvent{Type: provider.StreamUsage, Usage: usage}
	}
	if stopReason == "" && len(toolCallOrder) > 0 {
		stopReason = "tool_calls"
	}
	ch <- provider.StreamEvent{Type: provider.StreamDone, StopReason: stopReason}
}

func responsesToolKey(itemID string, outputIndex int) string {
	if itemID != "" {
		return itemID
	}
	return strconv.Itoa(outputIndex)
}

func convertResponsesUsage(u *responsesUsage) *provider.Usage {
	if u == nil {
		return nil
	}
	usage := &provider.Usage{Input: u.InputTokens, Output: u.OutputTokens, TotalTokens: u.TotalTokens}
	if u.InputTokensDetails != nil {
		usage.CacheRead = u.InputTokensDetails.CachedTokens
	}
	if u.OutputTokensDetails != nil {
		usage.Reasoning = u.OutputTokensDetails.ReasoningTokens
	}
	return usage
}

func responsesReasoningEffort(level provider.ThinkingLevel) string {
	switch level {
	case provider.ThinkingOff:
		return ""
	case provider.ThinkingMinimal:
		return "minimal"
	case provider.ThinkingLow:
		return "low"
	case provider.ThinkingMedium:
		return "medium"
	case provider.ThinkingHigh:
		return "high"
	case provider.ThinkingXHigh:
		return "high"
	default:
		return ""
	}
}

func (p *Provider) responsesReasoningSummary(model *provider.Model) string {
	if !supportsReasoningSummary(model) {
		return ""
	}
	if p.responsesConfig == nil {
		return "auto"
	}
	if p.responsesConfig.reasoningSummary == "none" || p.responsesConfig.reasoningSummary == "off" {
		return ""
	}
	if p.responsesConfig.reasoningSummary != "" {
		return p.responsesConfig.reasoningSummary
	}
	return "auto"
}

func (p *Provider) responsesPromptCacheKey(modelID string) string {
	if p.responsesConfig == nil {
		return ""
	}
	if p.responsesConfig.promptCacheKey != "" {
		return p.responsesConfig.promptCacheKey
	}
	if modelID == "" {
		return ""
	}
	return "vibecoding:" + strings.TrimPrefix(strings.TrimPrefix(p.baseURL, "https://"), "http://") + ":" + modelID
}

func supportsPromptCacheKey(model *provider.Model) bool {
	if model != nil && model.Compat != nil && model.Compat.SupportsPromptCacheKey != nil {
		return *model.Compat.SupportsPromptCacheKey
	}
	return true
}

func supportsPromptCacheRetention(model *provider.Model) bool {
	if model != nil && model.Compat != nil && model.Compat.SupportsLongCacheRetention != nil {
		return *model.Compat.SupportsLongCacheRetention
	}
	return true
}

func supportsReasoningSummary(model *provider.Model) bool {
	if model != nil && model.Compat != nil && model.Compat.SupportsReasoningSummary != nil {
		return *model.Compat.SupportsReasoningSummary
	}
	return true
}

func responseStopReason(status string) string {
	switch status {
	case "completed":
		return "stop"
	case "incomplete":
		return "length"
	case "failed":
		return "error"
	default:
		return status
	}
}
