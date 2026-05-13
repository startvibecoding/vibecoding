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

	"github.com/fuckvibecoding/vibecoding/internal/provider"
	"github.com/fuckvibecoding/vibecoding/internal/ua"
)

// Provider implements the Anthropic Messages API.
type Provider struct {
	provider.BaseProvider
	apiKey  string
	baseURL string
	client  *http.Client
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

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
	Thinking  *anthropicThinking `json:"thinking,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	Thinking  string                 `json:"thinking,omitempty"`
	Source    *anthropicImage        `json:"source,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   interface{}            `json:"content,omitempty"`
	IsError   bool                   `json:"is_error,omitempty"`
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
	Type         string          `json:"type"`
	Index        int             `json:"index,omitempty"`
	Delta        *anthropicDelta `json:"delta,omitempty"`
	ContentBlock *contentBlock   `json:"content_block,omitempty"`
	Message      *anthropicMsg   `json:"message,omitempty"`
	Usage        *anthropicUsage `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
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
			reqBody.System = params.SystemPrompt
		}

		if params.ThinkingLevel != provider.ThinkingOff {
			reqBody.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: thinkingBudget(params.ThinkingLevel)}
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("marshal: %w", err)}
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
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("send: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("API %d: %s", resp.StatusCode, string(b))}
			return
		}
		p.parseSSE(ctx, resp.Body, ch, params)
	}()
	return ch
}

func (p *Provider) parseSSE(ctx context.Context, body io.Reader, ch chan<- provider.StreamEvent, params provider.ChatParams) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		textContent      string
		reasonContent    string
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
			case "input_json_delta":
				if toolCallIndex >= 0 {
					if buf, ok := toolCallBuffers[toolCallIndex]; ok {
						buf.WriteString(event.Delta.PartialJSON)
					}
				}
			}
		case "content_block_stop":
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
				usage.Output = event.Usage.OutputTokens
			}
		}
	}

	if usage != nil {
		usage.TotalTokens = usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
		ch <- provider.StreamEvent{Type: provider.StreamUsage, Usage: usage}
	}
	ch <- provider.StreamEvent{Type: provider.StreamDone, StopReason: stopReason}
}

func (p *Provider) convertMessages(params provider.ChatParams) []anthropicMessage {
	var messages []anthropicMessage
	for _, msg := range params.Messages {
		am := anthropicMessage{Role: msg.Role}
		if msg.Role == "toolResult" {
			am.Role = "user"
			am.Content = []anthropicContentBlock{{Type: "tool_result", ToolUseID: msg.ToolCallID, Content: msg.Content, IsError: msg.IsError}}
		} else if len(msg.Contents) > 0 {
			var blocks []anthropicContentBlock
			for _, c := range msg.Contents {
				switch c.Type {
				case "text":
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: c.Text})
				case "image":
					if c.Image != nil {
						blocks = append(blocks, anthropicContentBlock{Type: "image", Source: &anthropicImage{Type: "base64", MediaType: c.Image.MimeType, Data: c.Image.Data}})
					}
				case "thinking":
					blocks = append(blocks, anthropicContentBlock{Type: "thinking", Thinking: c.Thinking})
				case "toolCall":
					if c.ToolCall != nil {
						input := make(map[string]interface{})
						if len(c.ToolCall.Arguments) > 0 {
							if err := json.Unmarshal(c.ToolCall.Arguments, &input); err != nil {
								// If unmarshal fails, use empty input
								input = make(map[string]interface{})
							}
						}
						blocks = append(blocks, anthropicContentBlock{Type: "tool_use", ID: c.ToolCall.ID, Name: c.ToolCall.Name, Input: input})
					}
				}
			}
			if len(blocks) == 1 && blocks[0].Type == "text" {
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

func (p *Provider) convertTools(tools []provider.ToolDefinition) []anthropicTool {
	var result []anthropicTool
	for _, t := range tools {
		result = append(result, anthropicTool{Name: t.Name, Description: t.Description, InputSchema: t.Parameters})
	}
	return result
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
