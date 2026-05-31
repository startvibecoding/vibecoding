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
	"strings"
	"time"

	"github.com/startvibecoding/vibecoding/internal/provider"
	"github.com/startvibecoding/vibecoding/internal/ua"
)

// Provider implements the OpenAI Chat Completions API.
type Provider struct {
	provider.BaseProvider
	apiKey  string
	baseURL string
	client  *http.Client

	// Configuration options
	disableReasoning bool   // Disable reasoning_content support for incompatible APIs
	thinkingFormat   string // "", "openai", "deepseek", "xiaomi"

	// Retry configuration
	retryConfig *provider.RetryConfig
}

// DefaultModels returns the default OpenAI model list.
func DefaultModels() []*provider.Model {
	return []*provider.Model{
		{
			ID: "gpt-4o", Name: "GPT-4o", Provider: "openai",
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 2.5, Output: 10.0, CacheRead: 1.25, CacheWrite: 2.5},
			ContextWindow: 128000, MaxTokens: 16384,
		},
		{
			ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai",
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 0.15, Output: 0.6, CacheRead: 0.075, CacheWrite: 0.15},
			ContextWindow: 128000, MaxTokens: 16384,
		},
		{
			ID: "o1", Name: "o1", Provider: "openai", Reasoning: true,
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 15.0, Output: 60.0, CacheRead: 7.5, CacheWrite: 15.0},
			ContextWindow: 200000, MaxTokens: 100000,
		},
		{
			ID: "o3-mini", Name: "o3-mini", Provider: "openai", Reasoning: true,
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 1.1, Output: 4.4, CacheRead: 0.55, CacheWrite: 1.1},
			ContextWindow: 200000, MaxTokens: 100000,
		},
	}
}

// NewProvider creates a new OpenAI provider with default models.
func NewProvider(apiKey, baseURL string) *Provider {
	return NewProviderWithModels(apiKey, baseURL, DefaultModels())
}

// NewProviderWithModels creates a new OpenAI provider with custom models.
func NewProviderWithModels(apiKey, baseURL string, models []*provider.Model) *Provider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	p := &Provider{
		BaseProvider: provider.NewBaseProvider("openai", models),
		apiKey:       apiKey,
		baseURL:      strings.TrimRight(baseURL, "/"),
		client:       &http.Client{Timeout: 30 * time.Minute},
	}

	// Check environment variable to disable reasoning
	if os.Getenv("OPENAI_DISABLE_REASONING") == "1" || os.Getenv("OPENAI_DISABLE_REASONING") == "true" {
		p.disableReasoning = true
	}

	return p
}

// DisableReasoning disables reasoning_content support for incompatible APIs.
func (p *Provider) DisableReasoning() {
	p.disableReasoning = true
}

// SetRetryConfig sets the retry configuration for this provider.
func (p *Provider) SetRetryConfig(cfg *provider.RetryConfig) {
	p.retryConfig = cfg
}

// IsReasoningDisabled returns whether reasoning support is disabled.
func (p *Provider) IsReasoningDisabled() bool {
	return p.disableReasoning
}

// SetThinkingFormat sets the thinking parameter format.
// "openai" = reasoning_effort, "deepseek" = thinking + reasoning_effort,
// "xiaomi" = legacy thinking-only format.
func (p *Provider) SetThinkingFormat(format string) {
	p.thinkingFormat = format
}

// openAIRequest represents the request body for OpenAI Chat Completions.
type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	Tools               []openAITool    `json:"tools,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Stream              bool            `json:"stream"`
	StreamOptions       *streamOptions  `json:"stream_options,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
	Thinking            *thinkingConfig `json:"thinking,omitempty"`
}

type thinkingConfig struct {
	Type string `json:"type"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content"`
	Reasoning  *string          `json:"reasoning_content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type openAIContentBlock struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *openAIImage `json:"image_url,omitempty"`
}

type openAIImage struct {
	URL string `json:"url"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIChoice       `json:"choices"`
	Usage   *openAIUsageResponse `json:"usage,omitempty"`
}

type openAIChoice struct {
	Index        int         `json:"index"`
	Delta        openAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type openAIDelta struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Reasoning *string          `json:"reasoning_content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls"`
}

type openAIUsageResponse struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

// Chat implements the streaming chat interface.
func (p *Provider) Chat(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
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
		messages := p.convertMessages(params, p.requiresReasoningContentOnAssistant(model))
		tools := p.convertTools(params.Tools)

		reqBody := openAIRequest{
			Model:         modelID,
			Messages:      messages,
			Tools:         tools,
			Stream:        true,
			StreamOptions: &streamOptions{IncludeUsage: true},
		}
		if maxTokensField(model) == "max_completion_tokens" {
			reqBody.MaxCompletionTokens = maxTokens
		} else {
			reqBody.MaxTokens = maxTokens
		}

		if !p.disableReasoning && params.ThinkingLevel != provider.ThinkingOff && model != nil && model.Reasoning {
			// Determine thinking format: explicit config > URL auto-detect > default
			format := p.thinkingFormatForModel(model)
			switch format {
			case "deepseek":
				reqBody.Thinking = &thinkingConfig{Type: "enabled"}
				if supportsReasoningEffort(model) {
					reqBody.ReasoningEffort = deepseekReasoningEffort(params.ThinkingLevel)
				}
			case "xiaomi":
				reqBody.Thinking = &thinkingConfig{Type: "enabled"}
			default: // "openai" or ""
				if supportsReasoningEffort(model) {
					reqBody.ReasoningEffort = openAIReasoningEffort(params.ThinkingLevel)
				}
			}
		}

		// Build the request body once (reused across retries)
		body, err := json.Marshal(reqBody)
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("marshal request: %w", err)}
			return
		}

		// Debug: dump request body
		if os.Getenv("VIBECODING_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] Request body: %s\n", string(body))
		}

		// Retry loop: retries only the initial HTTP connection, not the SSE stream.
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

			req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
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
					ch <- provider.StreamEvent{
						Type:         provider.StreamRetry,
						RetryAttempt: attempt + 1,
						RetryMax:     maxRetries,
						Error:        fmt.Errorf("%s", provider.FormatRetryMessage(attempt, maxRetries, delay, err)),
					}
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
					ch <- provider.StreamEvent{
						Type:         provider.StreamRetry,
						RetryAttempt: attempt + 1,
						RetryMax:     maxRetries,
						Error:        fmt.Errorf("%s", provider.FormatRetryMessage(attempt, maxRetries, delay, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)))),
					}
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

			// Success: stream the SSE response. No retry once streaming starts.
			p.parseSSE(ctx, resp.Body, ch, params)
			resp.Body.Close()
			return
		}

		// All retries exhausted (should not reach here with for..break logic, but safety net)
		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("all %d retry attempts exhausted", maxRetries)}
	}()

	return ch
}

func (p *Provider) parseSSE(ctx context.Context, body io.Reader, ch chan<- provider.StreamEvent, params provider.ChatParams) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		textContent     string
		reasonContent   string
		toolCalls       []provider.ToolCallBlock
		toolCallBuffers = make(map[int]*strings.Builder)
		stopReason      string
		usage           *provider.Usage
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

		var chunk openAIResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			// Only update usage if not already set (to avoid overwriting with partial values from different chunks)
			if usage == nil {
				usage = &provider.Usage{
					Input:       chunk.Usage.PromptTokens,
					Output:      chunk.Usage.CompletionTokens,
					TotalTokens: chunk.Usage.TotalTokens,
				}
				if chunk.Usage.PromptTokensDetails != nil {
					usage.CacheRead = chunk.Usage.PromptTokensDetails.CachedTokens
				}
			} else {
				// Update only if new values are provided and current values are 0
				if chunk.Usage.PromptTokens > 0 && usage.Input == 0 {
					usage.Input = chunk.Usage.PromptTokens
				}
				if chunk.Usage.CompletionTokens > 0 && usage.Output == 0 {
					usage.Output = chunk.Usage.CompletionTokens
				}
				if chunk.Usage.TotalTokens > 0 && usage.TotalTokens == 0 {
					usage.TotalTokens = chunk.Usage.TotalTokens
				}
				if chunk.Usage.PromptTokensDetails != nil && chunk.Usage.PromptTokensDetails.CachedTokens > 0 && usage.CacheRead == 0 {
					usage.CacheRead = chunk.Usage.PromptTokensDetails.CachedTokens
				}
			}
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				textContent += choice.Delta.Content
				ch <- provider.StreamEvent{Type: provider.StreamTextDelta, TextDelta: choice.Delta.Content}
			}
			if !p.disableReasoning && choice.Delta.Reasoning != nil && *choice.Delta.Reasoning != "" {
				reasonContent += *choice.Delta.Reasoning
				ch <- provider.StreamEvent{Type: provider.StreamThinkDelta, ThinkDelta: *choice.Delta.Reasoning}
			}
			for _, tc := range choice.Delta.ToolCalls {
				idx := tc.Index
				if idx < 0 {
					continue
				}
				if _, ok := toolCallBuffers[idx]; !ok {
					toolCallBuffers[idx] = &strings.Builder{}
					// Ensure slice is long enough
					for len(toolCalls) <= idx {
						toolCalls = append(toolCalls, provider.ToolCallBlock{})
					}
					toolCalls[idx].ID = tc.ID
					toolCalls[idx].Name = tc.Function.Name
				}
				if tc.ID != "" {
					toolCalls[idx].ID = tc.ID
				}
				if tc.Function.Name != "" {
					toolCalls[idx].Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCallBuffers[idx].WriteString(tc.Function.Arguments)
				}
			}
			if choice.FinishReason != nil {
				stopReason = *choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("stream read error: %w", err), StopReason: "error"}
		return
	}

	for i, tc := range toolCalls {
		if buf, ok := toolCallBuffers[i]; ok {
			if tc.ID == "" {
				// Some OpenAI-compatible providers omit tool call IDs in stream deltas.
				// Generate a stable fallback ID so subsequent tool results can always
				// bind to the corresponding assistant tool call.
				tc.ID = fmt.Sprintf("toolcall_%d", i)
			}
			tc.Arguments = json.RawMessage(buf.String())
			toolCalls[i] = tc
			ch <- provider.StreamEvent{Type: provider.StreamToolCall, ToolCall: &toolCalls[i]}
		}
	}

	if usage != nil {
		ch <- provider.StreamEvent{Type: provider.StreamUsage, Usage: usage}
	}
	ch <- provider.StreamEvent{Type: provider.StreamDone, StopReason: stopReason}
}

func openAIReasoningEffort(level provider.ThinkingLevel) string {
	switch level {
	case provider.ThinkingMinimal, provider.ThinkingLow:
		return "low"
	case provider.ThinkingMedium:
		return "medium"
	case provider.ThinkingHigh, provider.ThinkingXHigh:
		return "high"
	default:
		return ""
	}
}

func deepseekReasoningEffort(level provider.ThinkingLevel) string {
	switch level {
	case provider.ThinkingXHigh:
		return "max"
	default:
		return "high"
	}
}

func (p *Provider) thinkingFormatForModel(model *provider.Model) string {
	if p.thinkingFormat != "" {
		return p.thinkingFormat
	}
	if model != nil && model.Compat != nil && model.Compat.ThinkingFormat != "" {
		return model.Compat.ThinkingFormat
	}
	lowerBaseURL := strings.ToLower(p.baseURL)
	if strings.Contains(lowerBaseURL, "deepseek") {
		return "deepseek"
	}
	if strings.Contains(lowerBaseURL, "xiaomimimo") {
		return "xiaomi"
	}
	return ""
}

func supportsReasoningEffort(model *provider.Model) bool {
	if model != nil && model.Compat != nil && model.Compat.SupportsReasoningEffort != nil {
		return *model.Compat.SupportsReasoningEffort
	}
	return true
}

func maxTokensField(model *provider.Model) string {
	if model != nil && model.Compat != nil {
		return model.Compat.MaxTokensField
	}
	return ""
}

func (p *Provider) requiresReasoningContentOnAssistant(model *provider.Model) bool {
	if model != nil && model.Compat != nil && model.Compat.RequiresReasoningContentOnAssistant {
		return true
	}
	lowerBaseURL := strings.ToLower(p.baseURL)
	return strings.Contains(lowerBaseURL, "deepseek") || strings.Contains(lowerBaseURL, "xiaomimimo")
}

func (p *Provider) convertMessages(params provider.ChatParams, forceAssistantReasoning bool) []openAIMessage {
	var messages []openAIMessage

	// Add system prompt as the first message if provided
	if params.SystemPrompt != "" {
		messages = append(messages, openAIMessage{
			Role:    "system",
			Content: params.SystemPrompt,
		})
	}

	for _, msg := range params.Messages {
		om := openAIMessage{Role: msg.Role, ToolCallID: msg.ToolCallID}
		if msg.Role == "toolResult" {
			om.Role = "tool"
			if len(msg.Contents) > 0 {
				// Rich tool result: send text as tool message, images as supplementary user message
				om.Content = msg.Content
				messages = append(messages, om)
				// Collect image blocks for a supplementary user message
				var imageBlocks []openAIContentBlock
				for _, c := range msg.Contents {
					if c.Type == "image" && c.Image != nil {
						imageBlocks = append(imageBlocks, openAIContentBlock{Type: "image_url", ImageURL: &openAIImage{URL: fmt.Sprintf("data:%s;base64,%s", c.Image.MimeType, c.Image.Data)}})
					}
				}
				if len(imageBlocks) > 0 {
					// OpenAI tool messages can't contain images, so send them as a user message
					imageMsg := openAIMessage{Role: "user", Content: imageBlocks}
					messages = append(messages, imageMsg)
				}
				continue
			}
			om.Content = msg.Content
		} else if len(msg.Contents) > 0 {
			var blocks []openAIContentBlock
			var reasoningContent string
			for _, c := range msg.Contents {
				switch c.Type {
				case "text":
					blocks = append(blocks, openAIContentBlock{Type: "text", Text: c.Text})
				case "image":
					if c.Image != nil {
						blocks = append(blocks, openAIContentBlock{Type: "image_url", ImageURL: &openAIImage{URL: fmt.Sprintf("data:%s;base64,%s", c.Image.MimeType, c.Image.Data)}})
					}
				case "thinking":
					// Store reasoning content for OpenAI-compatible APIs
					if !p.disableReasoning {
						reasoningContent += c.Thinking
					}
				}
			}
			if len(blocks) == 1 && blocks[0].Type == "text" {
				om.Content = blocks[0].Text
			} else if len(blocks) > 0 {
				om.Content = blocks
			}
			// For assistant messages with tool calls, ensure content is not an empty array
			// Set reasoning content if available
			if reasoningContent != "" {
				om.Reasoning = &reasoningContent
			}
		} else {
			om.Content = msg.Content
		}
		if msg.Role == "assistant" {
			for _, c := range msg.Contents {
				if c.Type == "toolCall" && c.ToolCall != nil {
					om.ToolCalls = append(om.ToolCalls, openAIToolCall{ID: c.ToolCall.ID, Type: "function", Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: c.ToolCall.Name, Arguments: string(c.ToolCall.Arguments)}})
				}
			}
		}
		if msg.Role == "assistant" && forceAssistantReasoning && om.Reasoning == nil {
			reasoningContent := ""
			om.Reasoning = &reasoningContent
		}
		messages = append(messages, om)
	}
	return messages
}

func (p *Provider) convertTools(tools []provider.ToolDefinition) []openAITool {
	var result []openAITool
	for _, t := range tools {
		result = append(result, openAITool{Type: "function", Function: openAIFunction{Name: t.Name, Description: t.Description, Parameters: t.Parameters}})
	}
	return result
}
