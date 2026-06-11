package google

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

type APIKind string

const (
	APIKindGemini APIKind = "gemini"
	APIKindVertex APIKind = "vertex"
)

type Provider struct {
	provider.BaseProvider
	apiKey        string
	baseURL       string
	apiKind       APIKind
	client        *http.Client
	retryConfig   *provider.RetryConfig
	cachedContent string
	headers       map[string]string
}

func DefaultModels(providerName string) []*provider.Model {
	return []*provider.Model{
		{
			ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: providerName, Reasoning: true,
			Input: []string{"text", "image"}, ContextWindow: 1000000, MaxTokens: 65536,
		},
		{
			ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: providerName, Reasoning: true,
			Input: []string{"text", "image"}, ContextWindow: 1000000, MaxTokens: 65536,
		},
	}
}

func NewGeminiProvider(apiKey, baseURL string) *Provider {
	return NewGeminiProviderWithModels(apiKey, baseURL, DefaultModels("google-gemini"))
}

func NewGeminiProviderWithModels(apiKey, baseURL string, models []*provider.Model) *Provider {
	p, err := NewGeminiProviderWithModelsAndProxy(apiKey, baseURL, "", models)
	if err != nil {
		return newProviderWithHTTPClient("google-gemini", APIKindGemini, apiKey, baseURL, "https://generativelanguage.googleapis.com/v1beta/models", models, &http.Client{Timeout: 30 * time.Minute})
	}
	return p
}

func NewGeminiProviderWithModelsAndProxy(apiKey, baseURL, proxyURL string, models []*provider.Model) (*Provider, error) {
	return newProvider("google-gemini", APIKindGemini, apiKey, baseURL, "https://generativelanguage.googleapis.com/v1beta/models", proxyURL, models)
}

func NewVertexProvider(apiKey, baseURL string) *Provider {
	return NewVertexProviderWithModels(apiKey, baseURL, DefaultModels("google-vertex"))
}

func NewVertexProviderWithModels(apiKey, baseURL string, models []*provider.Model) *Provider {
	p, err := NewVertexProviderWithModelsAndProxy(apiKey, baseURL, "", models)
	if err != nil {
		return newProviderWithHTTPClient("google-vertex", APIKindVertex, apiKey, baseURL, "https://aiplatform.googleapis.com/v1/projects/YOUR_PROJECT/locations/global/publishers/google/models", models, &http.Client{Timeout: 30 * time.Minute})
	}
	return p
}

func NewVertexProviderWithModelsAndProxy(apiKey, baseURL, proxyURL string, models []*provider.Model) (*Provider, error) {
	return newProvider("google-vertex", APIKindVertex, apiKey, baseURL, "https://aiplatform.googleapis.com/v1/projects/YOUR_PROJECT/locations/global/publishers/google/models", proxyURL, models)
}

func newProvider(name string, kind APIKind, apiKey, baseURL, defaultBaseURL, proxyURL string, models []*provider.Model) (*Provider, error) {
	client, err := provider.NewHTTPClient(30*time.Minute, proxyURL)
	if err != nil {
		return nil, fmt.Errorf("configure http proxy: %w", err)
	}
	return newProviderWithHTTPClient(name, kind, apiKey, baseURL, defaultBaseURL, models, client), nil
}

func newProviderWithHTTPClient(name string, kind APIKind, apiKey, baseURL, defaultBaseURL string, models []*provider.Model, client *http.Client) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if apiKey == "" {
		switch kind {
		case APIKindGemini:
			apiKey = os.Getenv("GOOGLE_API_KEY")
		case APIKindVertex:
			apiKey = os.Getenv("GOOGLE_VERTEX_ACCESS_TOKEN")
		}
	}
	return &Provider{
		BaseProvider: provider.NewBaseProvider(name, models),
		apiKey:       apiKey,
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKind:      kind,
		client:       client,
	}
}

func (p *Provider) SetRetryConfig(cfg *provider.RetryConfig) {
	p.retryConfig = cfg
}

// SetHeaders sets custom HTTP headers applied to every provider request.
func (p *Provider) SetHeaders(headers map[string]string) {
	p.headers = cloneHeaders(headers)
}

// SetCachedContent sets an explicit Google cached content resource to reuse.
// The value should be a full cached content resource name, for example
// "cachedContents/abc123". Empty disables explicit cached content reuse.
func (p *Provider) SetCachedContent(name string) {
	p.cachedContent = strings.TrimSpace(name)
}

type googleRequest struct {
	SystemInstruction *googleContent        `json:"systemInstruction,omitempty"`
	Contents          []googleContent       `json:"contents"`
	Tools             []googleTool          `json:"tools,omitempty"`
	GenerationConfig  *googleGenerationConf `json:"generationConfig,omitempty"`
	CachedContent     string                `json:"cachedContent,omitempty"`
}

type googleGenerationConf struct {
	MaxOutputTokens int                   `json:"maxOutputTokens,omitempty"`
	Temperature     *float64              `json:"temperature,omitempty"`
	TopP            *float64              `json:"topP,omitempty"`
	ThinkingConfig  *googleThinkingConfig `json:"thinkingConfig,omitempty"`
}

type googleThinkingConfig struct {
	ThinkingBudget  int  `json:"thinkingBudget,omitempty"`
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
}

type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text             string                  `json:"text,omitempty"`
	Thought          bool                    `json:"thought,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
	InlineData       *googleInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *googleFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *googleFunctionResponse `json:"functionResponse,omitempty"`
}

type googleInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type googleFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type googleFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type googleTool struct {
	FunctionDeclarations []googleFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type googleFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type googleResponse struct {
	Candidates    []googleCandidate    `json:"candidates,omitempty"`
	UsageMetadata *googleUsageMetadata `json:"usageMetadata,omitempty"`
	Error         *googleResponseError `json:"error,omitempty"`
}

type googleCandidate struct {
	Content      googleContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type googleUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

type googleResponseError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
}

func (p *Provider) Chat(ctx context.Context, params provider.ChatParams) <-chan provider.StreamEvent {
	ch := make(chan provider.StreamEvent, 100)
	go func() {
		defer close(ch)

		if p.apiKey == "" {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("%s API key/token not set", p.Name())}
			return
		}

		modelID := params.ModelID
		if modelID == "" {
			if len(p.Models()) > 0 {
				modelID = p.Models()[0].ID
			} else {
				modelID = "gemini-2.5-flash"
			}
		}

		reqBody := googleRequest{
			Contents:         p.convertMessages(params),
			Tools:            p.convertTools(params.Tools),
			GenerationConfig: p.generationConfig(params, p.GetModel(modelID)),
		}
		if p.cachedContent != "" {
			reqBody.CachedContent = p.cachedContent
		}
		if params.SystemPrompt != "" {
			reqBody.SystemInstruction = &googleContent{Parts: []googlePart{{Text: params.SystemPrompt}}}
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("marshal request: %w", err)}
			return
		}
		if os.Getenv("VIBECODING_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] Google request body: %s\n", string(body))
		}

		maxRetries := 0
		baseDelayMs := 2000
		if p.retryConfig != nil && p.retryConfig.Enabled {
			maxRetries = p.retryConfig.MaxRetries
			baseDelayMs = p.retryConfig.BaseDelayMs
		}

		endpoint := p.streamEndpoint(modelID)
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if err := ctx.Err(); err != nil {
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: err, StopReason: "aborted"}
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
			if err != nil {
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("create request: %w", err)}
				return
			}
			p.setHeaders(req)

			resp, err := p.client.Do(req)
			if err != nil {
				if attempt < maxRetries && provider.IsRetryable(err, 0) {
					delay := provider.RetryDelay(attempt, baseDelayMs)
					ch <- provider.StreamEvent{Type: provider.StreamRetry, RetryAttempt: attempt + 1, RetryMax: maxRetries, Error: fmt.Errorf("%s", provider.FormatRetryMessage(attempt, maxRetries, delay, err))}
					if !sleepOrAbort(ctx, delay, ch) {
						return
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
					err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
					ch <- provider.StreamEvent{Type: provider.StreamRetry, RetryAttempt: attempt + 1, RetryMax: maxRetries, Error: fmt.Errorf("%s", provider.FormatRetryMessage(attempt, maxRetries, delay, err))}
					if !sleepOrAbort(ctx, delay, ch) {
						return
					}
					continue
				}
				ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))}
				return
			}

			p.parseSSE(ctx, resp.Body, ch, params)
			resp.Body.Close()
			return
		}
		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("all %d retry attempts exhausted", maxRetries)}
	}()
	return ch
}

func sleepOrAbort(ctx context.Context, delay time.Duration, ch chan<- provider.StreamEvent) bool {
	select {
	case <-ctx.Done():
		ch <- provider.StreamEvent{Type: provider.StreamError, Error: ctx.Err(), StopReason: "aborted"}
		return false
	case <-time.After(delay):
		return true
	}
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", ua.ProviderUserAgent())
	switch p.apiKind {
	case APIKindVertex:
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	default:
		req.Header.Set("x-goog-api-key", p.apiKey)
	}
	provider.ApplyHeaders(req, p.headers)
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	for name, value := range headers {
		cloned[name] = value
	}
	return cloned
}

func (p *Provider) streamEndpoint(modelID string) string {
	base := strings.TrimRight(p.baseURL, "/")
	model := strings.TrimPrefix(modelID, "models/")
	if strings.Contains(model, "/") {
		model = strings.Trim(model, "/")
	}
	return base + "/" + model + ":streamGenerateContent?alt=sse"
}

func (p *Provider) generationConfig(params provider.ChatParams, model *provider.Model) *googleGenerationConf {
	maxTokens := params.MaxTokens
	if maxTokens == 0 {
		maxTokens = 16384
	}
	cfg := &googleGenerationConf{
		MaxOutputTokens: maxTokens,
		Temperature:     params.Temperature,
		TopP:            params.TopP,
	}
	if params.ThinkingLevel != provider.ThinkingOff && model != nil && model.Reasoning {
		cfg.ThinkingConfig = &googleThinkingConfig{ThinkingBudget: googleThinkingBudget(params.ThinkingLevel), IncludeThoughts: true}
	}
	return cfg
}

func googleThinkingBudget(level provider.ThinkingLevel) int {
	switch level {
	case provider.ThinkingMinimal:
		return 128
	case provider.ThinkingLow:
		return 1024
	case provider.ThinkingHigh:
		return 8192
	case provider.ThinkingXHigh:
		return 24576
	default:
		return 4096
	}
}

func (p *Provider) convertMessages(params provider.ChatParams) []googleContent {
	var contents []googleContent
	for _, msg := range params.Messages {
		content := googleContent{Role: googleRole(msg.Role)}
		if msg.Role == "toolResult" {
			response := map[string]any{"content": msg.Content}
			if msg.IsError {
				response["error"] = true
			}
			content.Parts = append(content.Parts, googlePart{FunctionResponse: &googleFunctionResponse{Name: msg.ToolName, Response: response}})
			contents = append(contents, content)
			continue
		}

		if len(msg.Contents) == 0 {
			if msg.Content != "" {
				content.Parts = append(content.Parts, googlePart{Text: msg.Content})
			}
			if len(content.Parts) > 0 {
				contents = append(contents, content)
			}
			continue
		}

		for _, block := range msg.Contents {
			switch block.Type {
			case "text":
				if block.Text != "" {
					content.Parts = append(content.Parts, googlePart{Text: block.Text})
				}
			case "image":
				if block.Image != nil {
					content.Parts = append(content.Parts, googlePart{InlineData: &googleInlineData{MimeType: block.Image.MimeType, Data: block.Image.Data}})
				}
			case "toolCall":
				if block.ToolCall != nil {
					content.Parts = append(content.Parts, googlePart{FunctionCall: &googleFunctionCall{Name: block.ToolCall.Name, Args: block.ToolCall.Arguments}})
				}
			}
		}
		if len(content.Parts) > 0 {
			contents = append(contents, content)
		}
	}
	return contents
}

func googleRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "toolResult":
		return "user"
	default:
		return "user"
	}
}

func (p *Provider) convertTools(tools []provider.ToolDefinition) []googleTool {
	var declarations []googleFunctionDeclaration
	for _, t := range tools {
		if t.Kind == "hosted" {
			continue
		}
		declarations = append(declarations, googleFunctionDeclaration{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	if len(declarations) == 0 {
		return nil
	}
	return []googleTool{{FunctionDeclarations: declarations}}
}

func (p *Provider) parseSSE(ctx context.Context, body io.Reader, ch chan<- provider.StreamEvent, params provider.ChatParams) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	ch <- provider.StreamEvent{Type: provider.StreamStart}
	var usage *provider.Usage
	var stopReason string
	toolCallIndex := 0

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

		var chunk googleResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("%s: %s", chunk.Error.Status, chunk.Error.Message), StopReason: "error"}
			return
		}
		if chunk.UsageMetadata != nil {
			usage = convertUsage(chunk.UsageMetadata)
		}

		for _, candidate := range chunk.Candidates {
			if candidate.FinishReason != "" {
				stopReason = strings.ToLower(candidate.FinishReason)
			}
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					if part.Thought {
						ch <- provider.StreamEvent{Type: provider.StreamThinkDelta, ThinkDelta: part.Text}
					} else {
						ch <- provider.StreamEvent{Type: provider.StreamTextDelta, TextDelta: part.Text}
					}
				}
				if part.ThoughtSignature != "" {
					ch <- provider.StreamEvent{Type: provider.StreamThinkSignature, ThinkSignature: part.ThoughtSignature}
				}
				if part.FunctionCall != nil {
					toolCallIndex++
					args := part.FunctionCall.Args
					if len(args) == 0 {
						args = json.RawMessage(`{}`)
					}
					tc := &provider.ToolCallBlock{
						ID:        fmt.Sprintf("google_toolcall_%d", toolCallIndex),
						Name:      part.FunctionCall.Name,
						Arguments: args,
					}
					ch <- provider.StreamEvent{Type: provider.StreamToolCall, ToolCall: tc}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- provider.StreamEvent{Type: provider.StreamError, Error: fmt.Errorf("stream read error: %w", err), StopReason: "error"}
		return
	}
	if usage != nil {
		ch <- provider.StreamEvent{Type: provider.StreamUsage, Usage: usage}
	}
	ch <- provider.StreamEvent{Type: provider.StreamDone, StopReason: stopReason}
}

func convertUsage(u *googleUsageMetadata) *provider.Usage {
	if u == nil {
		return nil
	}
	return &provider.Usage{
		Input:       u.PromptTokenCount,
		Output:      u.CandidatesTokenCount,
		Reasoning:   u.ThoughtsTokenCount,
		CacheRead:   u.CachedContentTokenCount,
		TotalTokens: u.TotalTokenCount,
	}
}
