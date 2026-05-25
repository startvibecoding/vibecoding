package anthropic

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

// Provider implements the Anthropic Messages API.
type Provider struct {
	provider.BaseProvider
	apiKey  string
	baseURL string
	client  *http.Client

	thinkingFormat      string // "", "anthropic", "deepseek", "xiaomi"
	cacheControlEnabled *bool  // nil=off (must be explicitly enabled), true=on, false=off

	// Retry configuration
	retryConfig *provider.RetryConfig
}

// DefaultModels returns the default Anthropic model list.
func DefaultModels() []*provider.Model {
	return []*provider.Model{
		{
			ID: "claude-sonnet-4-20250514", Name: "Claude 4 Sonnet", Provider: "anthropic", Reasoning: true,
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 3.75},
			ContextWindow: 200000, MaxTokens: 16384,
		},
		{
			ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Provider: "anthropic",
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 3.75},
			ContextWindow: 200000, MaxTokens: 8192,
		},
		{
			ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Provider: "anthropic",
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 0.8, Output: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
			ContextWindow: 200000, MaxTokens: 8192,
		},
		{
			ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic",
			Input: []string{"text", "image"}, Cost: provider.ModelPricing{Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 18.75},
			ContextWindow: 200000, MaxTokens: 4096,
		},
	}
}

// NewProvider creates a new Anthropic provider with default models.
func NewProvider(apiKey, baseURL string) *Provider {
	return NewProviderWithModels(apiKey, baseURL, DefaultModels())
}

// NewProviderWithModels creates a new Anthropic provider with custom models.
func NewProviderWithModels(apiKey, baseURL string, models []*provider.Model) *Provider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	return &Provider{
		BaseProvider: provider.NewBaseProvider("anthropic", models),
		apiKey:       apiKey,
		baseURL:      strings.TrimRight(baseURL, "/"),
		client:       &http.Client{Timeout: 30 * time.Minute},
	}
}

// SetThinkingFormat sets the thinking parameter format.
// "anthropic" = thinking with budget_tokens, "deepseek" = thinking with output_config,
// "xiaomi" = legacy thinking-only format.
func (p *Provider) SetThinkingFormat(format string) {
	p.thinkingFormat = format
}

// SetRetryConfig sets the retry configuration for this provider.
func (p *Provider) SetRetryConfig(cfg *provider.RetryConfig) {
	p.retryConfig = cfg
}

// SetCacheControlEnabled sets whether to use cache_control markers.
// nil = off (default), true = on, false = off
func (p *Provider) SetCacheControlEnabled(enabled *bool) {
	p.cacheControlEnabled = enabled
}

// IsCacheControlEnabled returns whether cache_control markers should be used.
// Must be explicitly enabled via SetCacheControlEnabled or provider config "cacheControl": true.
// Defaults to false when not configured.
func (p *Provider) IsCacheControlEnabled() bool {
	if p.cacheControlEnabled != nil {
		return *p.cacheControlEnabled
	}
	return false
}

type anthropicRequest struct {
	Model        string                 `json:"model"`
	Messages     []anthropicMessage     `json:"messages"`
	System       interface{}            `json:"system,omitempty"` // string or []anthropicContentBlock for cache_control
	Tools        []anthropicTool        `json:"tools,omitempty"`
	MaxTokens    int                    `json:"max_tokens"`
	Stream       bool                   `json:"stream"`
	Thinking     *anthropicThinking     `json:"thinking,omitempty"`
	OutputConfig *anthropicOutputConfig `json:"output_config,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens *int   `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"`
}

type anthropicOutputConfig struct {
	Effort string `json:"effort"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type anthropicContentBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Thinking     string                 `json:"thinking,omitempty"`
	Signature    string                 `json:"signature,omitempty"`
	Source       *anthropicImage        `json:"source,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Input        *map[string]interface{} `json:"input,omitempty"`
	ToolUseID    string                 `json:"tool_use_id,omitempty"`
	Content      interface{}            `json:"content,omitempty"`
	IsError      bool                   `json:"is_error,omitempty"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicImage struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicResponse struct {
	Type         string                 `json:"type"`
	Index        int                    `json:"index,omitempty"`
	Delta        *anthropicDelta        `json:"delta,omitempty"`
	ContentBlock *contentBlock          `json:"content_block,omitempty"`
	Message      *anthropicMsg          `json:"message,omitempty"`
	Usage        *anthropicUsage        `json:"usage,omitempty"`
	Error        *anthropicStreamError  `json:"error,omitempty"`
}

type anthropicStreamError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	Signature   string `json:"signature,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type anthropicMsg struct {
	ID         string          `json:"id"`
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason"`
	Usage      *anthropicUsage `json:"usage"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Chat implements the streaming chat interface.
func (p *Provider) Chat(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
	ch := make(chan provider.StreamEvent, 100)
	go func() {
		defer close(ch)
		if p.apiKey == "" {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("ANTHROPIC_API_KEY not set")}
			return
		}

		modelID := params.ModelID
		if modelID == "" {
			if len(p.Models()) > 0 {
				modelID = p.Models()[0].ID
			} else {
				modelID = "claude-sonnet-4-20250514"
			}
		}
		model := p.GetModel(modelID)

		maxTokens := params.MaxTokens
		if maxTokens == 0 {
			maxTokens = 16384
		}

		reqBody := anthropicRequest{
			Model:     modelID,
			Messages:  p.convertMessages(params),
			Tools:     p.convertTools(params.Tools),
			MaxTokens: maxTokens,
			Stream:    true,
		}
		if params.SystemPrompt != "" {
			if p.IsCacheControlEnabled() {
				// Send system prompt as content block array with cache_control for prompt caching
				sysBlock := anthropicContentBlock{
					Type:         "text",
					Text:         params.SystemPrompt,
					CacheControl: &anthropicCacheControl{Type: "ephemeral"},
				}
				reqBody.System = []anthropicContentBlock{sysBlock}
			} else {
				// Send system prompt as simple string (for proxies that don't support array format)
				reqBody.System = params.SystemPrompt
			}
		}

		if params.ThinkingLevel != provider.ThinkingOff && model != nil && model.Reasoning {
			// Determine thinking format: explicit config > URL auto-detect > default
			format := p.thinkingFormat
			if format == "" {
				lowerBaseURL := strings.ToLower(p.baseURL)
				if strings.Contains(lowerBaseURL, "deepseek") {
					format = "deepseek"
				} else if strings.Contains(lowerBaseURL, "xiaomimimo") {
					format = "xiaomi"
				}
			}
			switch format {
			case "deepseek":
				reqBody.Thinking = &anthropicThinking{Type: "enabled"}
				reqBody.OutputConfig = &anthropicOutputConfig{Effort: deepseekReasoningEffort(params.ThinkingLevel)}
			case "xiaomi":
				reqBody.Thinking = &anthropicThinking{Type: "enabled"}
			case "adaptive":
				reqBody.Thinking = &anthropicThinking{Type: "adaptive", Display: "summarized"}
				reqBody.OutputConfig = &anthropicOutputConfig{Effort: anthropicAdaptiveEffort(params.ThinkingLevel)}
			default: // "anthropic" or ""
				if isAnthropicAdaptiveModel(modelID) {
					reqBody.Thinking = &anthropicThinking{Type: "adaptive", Display: "summarized"}
					reqBody.OutputConfig = &anthropicOutputConfig{Effort: anthropicAdaptiveEffort(params.ThinkingLevel)}
				} else {
					budget := thinkingBudget(params.ThinkingLevel)
					reqBody.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: &budget}
				}
			}
		}

		// Build the request body once (reused across retries)
		body, err := json.Marshal(reqBody)
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("marshal: %w", err)}
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

			req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(body))
			if err != nil {
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("request: %w", err)}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", p.apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
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
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("send: %w", err)}
				return
			}

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if os.Getenv("VIBECODING_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "[DEBUG] API Error %d: %s\n", resp.StatusCode, string(b))
					fmt.Fprintf(os.Stderr, "[DEBUG] Request body was: %s\n", string(body))
				}
				if attempt < maxRetries && provider.IsRetryable(nil, resp.StatusCode) {
					delay := provider.RetryDelay(attempt, baseDelayMs)
					ch <- provider.StreamEvent{
						Type:         provider.StreamRetry,
						RetryAttempt: attempt + 1,
						RetryMax:     maxRetries,
						Error:        fmt.Errorf("%s", provider.FormatRetryMessage(attempt, maxRetries, delay, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b)))),
					}
					select {
					case <-ctx.Done():
						ch <- provider.StreamEvent{Type: provider.StreamError, Error: ctx.Err(), StopReason: "aborted"}
						return
					case <-time.After(delay):
					}
					continue
				}
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("API %d: %s", resp.StatusCode, string(b))}
				return
			}

			// Success: stream the SSE response. No retry once streaming starts.
			p.parseSSE(ctx, resp.Body, ch, params)
			resp.Body.Close()
			return
		}

		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("all %d retry attempts exhausted", maxRetries)}
	}()
	return ch
}

func (p *Provider) parseSSE(ctx context.Context, body io.Reader, ch chan<- provider.StreamEvent, params provider.ChatParams) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		textContent      string
		reasonContent    string
		thinkSignature   string
		toolCalls        []provider.ToolCallBlock
		toolCallBuffers  = make(map[int]*strings.Builder)
		stopReason       string
		usage            *provider.Usage
		currentBlockType string
		toolCallIndex    int = -1
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

		var event anthropicResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil && event.Message.Usage != nil {
				usage = &provider.Usage{
					Input: event.Message.Usage.InputTokens, Output: event.Message.Usage.OutputTokens,
					CacheRead: event.Message.Usage.CacheReadInputTokens, CacheWrite: event.Message.Usage.CacheCreationInputTokens,
				}
			}
		case "content_block_start":
			if event.ContentBlock != nil {
				currentBlockType = event.ContentBlock.Type
				if event.ContentBlock.Type == "tool_use" {
					toolCallIndex = len(toolCalls)
					toolCalls = append(toolCalls, provider.ToolCallBlock{ID: event.ContentBlock.ID, Name: event.ContentBlock.Name})
					toolCallBuffers[toolCallIndex] = &strings.Builder{}
				}
			}
		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				textContent += event.Delta.Text
				ch <- provider.StreamEvent{Type: provider.StreamTextDelta, TextDelta: event.Delta.Text}
			case "thinking_delta":
				reasonContent += event.Delta.Thinking
				ch <- provider.StreamEvent{Type: provider.StreamThinkDelta, ThinkDelta: event.Delta.Thinking}
			case "signature_delta":
				thinkSignature += event.Delta.Signature
			case "input_json_delta":
				if toolCallIndex >= 0 {
					if buf, ok := toolCallBuffers[toolCallIndex]; ok {
						buf.WriteString(event.Delta.PartialJSON)
					}
				}
			}
		case "content_block_stop":
			if currentBlockType == "thinking" && thinkSignature != "" {
				ch <- provider.StreamEvent{Type: provider.StreamThinkSignature, ThinkSignature: thinkSignature}
				thinkSignature = ""
			}
			if currentBlockType == "tool_use" && toolCallIndex >= 0 && toolCallIndex < len(toolCalls) {
				if buf, ok := toolCallBuffers[toolCallIndex]; ok {
					toolCalls[toolCallIndex].Arguments = json.RawMessage(buf.String())
					ch <- provider.StreamEvent{Type: provider.StreamToolCall, ToolCall: &toolCalls[toolCallIndex]}
				}
			}
			toolCallIndex = -1
		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				stopReason = event.Delta.StopReason
			}
			if event.Usage != nil {
				if usage == nil {
					usage = &provider.Usage{}
				}
				// Some proxies send all usage data in message_delta instead of message_start.
				// Only update values if they haven't been set yet (to avoid overwriting with partial values).
				if event.Usage.OutputTokens > 0 && usage.Output == 0 {
					usage.Output = event.Usage.OutputTokens
				}
				if event.Usage.InputTokens > 0 && usage.Input == 0 {
					usage.Input = event.Usage.InputTokens
				}
				if event.Usage.CacheReadInputTokens > 0 && usage.CacheRead == 0 {
					usage.CacheRead = event.Usage.CacheReadInputTokens
				}
				if event.Usage.CacheCreationInputTokens > 0 && usage.CacheWrite == 0 {
					usage.CacheWrite = event.Usage.CacheCreationInputTokens
				}
			}
		case "error":
			errMsg := "stream error"
			if event.Error != nil {
				errMsg = event.Error.Message
				if event.Error.Type != "" {
					errMsg = event.Error.Type + ": " + errMsg
				}
			}
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("%s", errMsg), StopReason: "error"}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("stream read error: %w", err), StopReason: "error"}
		return
	}

	if usage != nil {
		usage.TotalTokens = usage.Input + usage.CacheRead + usage.CacheWrite + usage.Output
		ch <- provider.StreamEvent{Type: provider.StreamUsage, Usage: usage}
	}
	ch <- provider.StreamEvent{Type: provider.StreamDone, StopReason: stopReason}
}

func (p *Provider) convertMessages(params provider.ChatParams) []anthropicMessage {
	cacheEnabled := p.IsCacheControlEnabled()
	var messages []anthropicMessage
	for i := 0; i < len(params.Messages); i++ {
		msg := params.Messages[i]
		am := anthropicMessage{Role: msg.Role}
		if msg.Role == "toolResult" {
			// Anthropic requires all tool_result blocks for the preceding assistant
			// tool_use blocks to be in the next user message, before any other
			// content. Group consecutive tool results to preserve that shape.
			blocks, next := p.convertToolResultRun(params.Messages, i, cacheEnabled)
			messages = append(messages, anthropicMessage{Role: "user", Content: blocks})
			i = next - 1
			continue
		} else if len(msg.Contents) > 0 {
			var blocks []anthropicContentBlock
			for _, c := range msg.Contents {
				block := anthropicContentBlock{}
				switch c.Type {
				case "text":
					block = anthropicContentBlock{Type: "text", Text: c.Text}
				case "image":
					if c.Image != nil {
						block = anthropicContentBlock{Type: "image", Source: &anthropicImage{Type: "base64", MediaType: c.Image.MimeType, Data: c.Image.Data}}
					}
				case "thinking":
					block = anthropicContentBlock{Type: "thinking", Thinking: c.Thinking, Signature: c.Signature}
				case "toolCall":
					if c.ToolCall != nil {
						input := make(map[string]interface{})
						if len(c.ToolCall.Arguments) > 0 {
							if err := json.Unmarshal(c.ToolCall.Arguments, &input); err != nil {
								fmt.Fprintf(os.Stderr, "Warning: failed to unmarshal tool call arguments for %s: %v\n", c.ToolCall.Name, err)
								input = make(map[string]interface{})
							}
						}
						block = anthropicContentBlock{Type: "tool_use", ID: c.ToolCall.ID, Name: c.ToolCall.Name, Input: &input}
					}
				}
				// Pass through cache_control from provider content blocks (only if enabled)
				if c.CacheControl != nil && cacheEnabled {
					block.CacheControl = &anthropicCacheControl{Type: c.CacheControl.Type}
				}
				blocks = append(blocks, block)
			}
			if len(blocks) == 1 && blocks[0].Type == "text" && blocks[0].CacheControl == nil {
				am.Content = blocks[0].Text
			} else {
				am.Content = blocks
			}
		} else {
			am.Content = msg.Content
		}
		messages = append(messages, am)
	}
	return messages
}

func (p *Provider) convertToolResultRun(messages []provider.Message, start int, cacheEnabled bool) ([]anthropicContentBlock, int) {
	var resultBlocks []anthropicContentBlock
	var imageBlocks []anthropicContentBlock
	i := start
	for i < len(messages) && messages[i].Role == "toolResult" {
		resultBlock, images := p.convertToolResultMessage(messages[i], cacheEnabled)
		resultBlocks = append(resultBlocks, resultBlock)
		imageBlocks = append(imageBlocks, images...)
		i++
	}
	return append(resultBlocks, imageBlocks...), i
}

func (p *Provider) convertToolResultMessage(msg provider.Message, cacheEnabled bool) (anthropicContentBlock, []anthropicContentBlock) {
	textContent := msg.Content
	var imageBlocks []anthropicContentBlock
	var hasCacheControl bool

	if len(msg.Contents) > 0 {
		var textParts []string
		for _, c := range msg.Contents {
			switch c.Type {
			case "text":
				if c.Text != "" {
					textParts = append(textParts, c.Text)
				}
				if c.CacheControl != nil {
					hasCacheControl = true
				}
			case "image":
				if c.Image != nil {
					imageBlocks = append(imageBlocks, anthropicContentBlock{Type: "image", Source: &anthropicImage{Type: "base64", MediaType: c.Image.MimeType, Data: c.Image.Data}})
				}
			}
		}
		if len(textParts) > 0 {
			textContent = strings.Join(textParts, "\n")
		}
	}

	if strings.TrimSpace(textContent) == "" {
		textContent = "Tool completed with no output."
	}

	resultBlock := anthropicContentBlock{Type: "tool_result", ToolUseID: msg.ToolCallID, Content: textContent, IsError: msg.IsError}
	if hasCacheControl && cacheEnabled {
		resultBlock.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
	}
	return resultBlock, imageBlocks
}

func (p *Provider) convertTools(tools []provider.ToolDefinition) []anthropicTool {
	var result []anthropicTool
	for _, t := range tools {
		result = append(result, anthropicTool{Name: t.Name, Description: t.Description, InputSchema: t.Parameters})
	}
	return result
}

func deepseekReasoningEffort(level provider.ThinkingLevel) string {
	switch level {
	case provider.ThinkingXHigh:
		return "max"
	default:
		return "high"
	}
}

func isAnthropicAdaptiveModel(modelID string) bool {
	return strings.HasPrefix(modelID, "claude-opus-4-7") ||
		strings.HasPrefix(modelID, "claude-opus-4-6") ||
		strings.HasPrefix(modelID, "claude-sonnet-4-6")
}

func anthropicAdaptiveEffort(level provider.ThinkingLevel) string {
	switch level {
	case provider.ThinkingMinimal, provider.ThinkingLow:
		return "low"
	case provider.ThinkingMedium:
		return "medium"
	case provider.ThinkingHigh:
		return "high"
	case provider.ThinkingXHigh:
		return "xhigh"
	default:
		return "high"
	}
}

func thinkingBudget(level provider.ThinkingLevel) int {
	switch level {
	case provider.ThinkingMinimal:
		return 1024
	case provider.ThinkingLow:
		return 4096
	case provider.ThinkingMedium:
		return 10240
	case provider.ThinkingHigh:
		return 32768
	case provider.ThinkingXHigh:
		return 65536
	default:
		return 10240
	}
}
