package google

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/config"
	"github.com/startvibecoding/vibecoding/internal/provider"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newMockGoogleProvider(t *testing.T, p *Provider, sse string, bodyCh chan<- string, check func(*http.Request)) *Provider {
	t.Helper()
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if check != nil {
			check(r)
		}
		if bodyCh != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			bodyCh <- string(body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(sse)),
			Request:    r,
		}, nil
	})}
	return p
}

func TestResolveAPIKeyShellCommandRequiresOptIn(t *testing.T) {
	t.Setenv("VIBECODING_ALLOW_SHELL_CONFIG", "")
	if got := resolveAPIKey(&config.ProviderConfig{APIKey: "!printf secret"}); got != "!printf secret" {
		t.Fatalf("resolveAPIKey without opt-in = %q, want literal", got)
	}

	t.Setenv("VIBECODING_ALLOW_SHELL_CONFIG", "1")
	if got := resolveAPIKey(&config.ProviderConfig{APIKey: "!printf secret"}); got != "secret" {
		t.Fatalf("resolveAPIKey with opt-in = %q, want secret", got)
	}
}

func TestGoogleProviderHTTPProxy(t *testing.T) {
	p, err := NewGeminiProviderWithModelsAndProxy("fake-key", "https://generativelanguage.googleapis.com/v1beta/models", "http://127.0.0.1:7890", []*provider.Model{{ID: "m1"}})
	if err != nil {
		t.Fatalf("provider with proxy: %v", err)
	}
	transport, ok := p.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", p.client.Transport)
	}
	proxyURL, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "generativelanguage.googleapis.com"}})
	if err != nil {
		t.Fatalf("proxy lookup: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("proxy = %v, want http://127.0.0.1:7890", proxyURL)
	}
}

func TestConvertMessagesToolResultUsesTextContents(t *testing.T) {
	p := &Provider{}
	contents := p.convertMessages(provider.ChatParams{
		Messages: []provider.Message{
			{
				Role:       "toolResult",
				ToolCallID: "call_1",
				ToolName:   "bash",
				Contents: []provider.ContentBlock{
					{Type: "text", Text: "bash output from content block", CacheControl: &provider.CacheControl{Type: "ephemeral"}},
				},
			},
		},
	})

	if len(contents) != 1 || len(contents[0].Parts) != 1 || contents[0].Parts[0].FunctionResponse == nil {
		t.Fatalf("contents = %#v, want one function response", contents)
	}
	got := contents[0].Parts[0].FunctionResponse.Response["content"]
	if got != "bash output from content block" {
		t.Fatalf("function response content = %#v, want text content from content block", got)
	}
}

func TestGoogleCustomHeaders(t *testing.T) {
	p := newMockGoogleProvider(t,
		NewGeminiProviderWithModels("fake-key", "https://generativelanguage.googleapis.com/v1beta/models", []*provider.Model{{ID: "gemini-test"}}),
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]},\"finishReason\":\"STOP\"}]}\n",
		nil,
		func(r *http.Request) {
			if r.Header.Get("X-Custom-Header") != "custom-value" {
				t.Fatalf("X-Custom-Header = %q, want custom-value", r.Header.Get("X-Custom-Header"))
			}
			if r.Header.Get("x-goog-api-key") != "override-key" {
				t.Fatalf("x-goog-api-key = %q, want override-key", r.Header.Get("x-goog-api-key"))
			}
		})
	p.SetHeaders(map[string]string{
		"X-Custom-Header": "custom-value",
		"x-goog-api-key":  "override-key",
	})

	params := provider.ChatParams{
		ModelID:  "gemini-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}
}

func TestGoogleGeminiRequest(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockGoogleProvider(t,
		NewGeminiProviderWithModels("fake-key", "https://generativelanguage.googleapis.com/v1beta/models", []*provider.Model{{ID: "gemini-test", Reasoning: true}}),
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]},\"finishReason\":\"STOP\"}]}\n",
		bodyCh,
		func(r *http.Request) {
			if r.URL.Path != "/v1beta/models/gemini-test:streamGenerateContent" {
				t.Fatalf("path = %q, want /v1beta/models/gemini-test:streamGenerateContent", r.URL.Path)
			}
			if r.URL.Query().Get("alt") != "sse" {
				t.Fatalf("alt query = %q, want sse", r.URL.Query().Get("alt"))
			}
			if r.Header.Get("x-goog-api-key") != "fake-key" {
				t.Fatalf("x-goog-api-key = %q, want fake-key", r.Header.Get("x-goog-api-key"))
			}
		})

	temp := 0.2
	params := provider.ChatParams{
		ModelID:       "gemini-test",
		SystemPrompt:  "system",
		Messages:      []provider.Message{provider.NewUserMessage("hi")},
		Tools:         []provider.ToolDefinition{{Name: "read", Description: "Read file", Parameters: json.RawMessage(`{"type":"object"}`)}},
		ThinkingLevel: provider.ThinkingHigh,
		MaxTokens:     123,
		Temperature:   &temp,
		Abort:         make(chan struct{}),
	}
	for range p.Chat(context.Background(), params) {
	}

	var req googleRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}
	if req.SystemInstruction == nil || req.SystemInstruction.Parts[0].Text != "system" {
		t.Fatalf("systemInstruction = %#v, want system text", req.SystemInstruction)
	}
	if len(req.Contents) != 1 || req.Contents[0].Role != "user" || req.Contents[0].Parts[0].Text != "hi" {
		t.Fatalf("contents = %#v, want user hi", req.Contents)
	}
	if req.GenerationConfig == nil || req.GenerationConfig.MaxOutputTokens != 123 {
		t.Fatalf("generationConfig = %#v, want max 123", req.GenerationConfig)
	}
	if req.GenerationConfig.Temperature == nil || *req.GenerationConfig.Temperature != temp {
		t.Fatalf("temperature = %#v, want %v", req.GenerationConfig.Temperature, temp)
	}
	if req.GenerationConfig.ThinkingConfig == nil || req.GenerationConfig.ThinkingConfig.ThinkingBudget != 8192 {
		t.Fatalf("thinkingConfig = %#v, want high budget", req.GenerationConfig.ThinkingConfig)
	}
	if !req.GenerationConfig.ThinkingConfig.IncludeThoughts {
		t.Fatal("thinkingConfig.includeThoughts = false, want true")
	}
	if len(req.Tools) != 1 || len(req.Tools[0].FunctionDeclarations) != 1 || req.Tools[0].FunctionDeclarations[0].Name != "read" {
		t.Fatalf("tools = %#v, want read declaration", req.Tools)
	}
}

func TestGoogleRequestCachedContent(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := NewGeminiProviderWithModels("fake-key", "https://generativelanguage.googleapis.com/v1beta/models", []*provider.Model{{ID: "gemini-test"}})
	p.SetCachedContent("cachedContents/test-cache")
	p = newMockGoogleProvider(t, p, "data: {}\n", bodyCh, nil)

	for range p.Chat(context.Background(), provider.ChatParams{
		ModelID:  "gemini-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}) {
	}

	var req googleRequest
	select {
	case body := <-bodyCh:
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal request body: %v\nbody: %s", err, body)
		}
	default:
		t.Fatal("no request body captured")
	}

	if req.CachedContent != "cachedContents/test-cache" {
		t.Fatalf("cachedContent = %q, want cachedContents/test-cache", req.CachedContent)
	}
}

func TestGoogleVertexAPIKeyHeaderAndEndpoint(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockGoogleProvider(t,
		NewVertexProviderWithModels("fake-key", "https://aiplatform.googleapis.com/v1/projects/test/locations/global/publishers/google/models", []*provider.Model{{ID: "gemini-test"}}),
		"data: {}\n",
		bodyCh,
		func(r *http.Request) {
			if r.URL.Path != "/v1/publishers/google/models/gemini-test:streamGenerateContent" {
				t.Fatalf("path = %q, want Vertex API key streamGenerateContent path", r.URL.Path)
			}
			if r.Header.Get("x-goog-api-key") != "fake-key" {
				t.Fatalf("x-goog-api-key = %q, want fake-key", r.Header.Get("x-goog-api-key"))
			}
			if r.Header.Get("Authorization") != "" {
				t.Fatalf("Authorization = %q, want empty", r.Header.Get("Authorization"))
			}
		})

	for range p.Chat(context.Background(), provider.ChatParams{
		ModelID:  "gemini-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}) {
	}
}

func TestGoogleVertexOAuthAuthorizationHeader(t *testing.T) {
	bodyCh := make(chan string, 1)
	p := newMockGoogleProvider(t,
		NewVertexProviderWithModels("ya29.fake-token", "https://aiplatform.googleapis.com/v1/projects/test/locations/global/publishers/google/models", []*provider.Model{{ID: "gemini-test"}}),
		"data: {}\n",
		bodyCh,
		func(r *http.Request) {
			if r.URL.Path != "/v1/projects/test/locations/global/publishers/google/models/gemini-test:streamGenerateContent" {
				t.Fatalf("path = %q, want Vertex OAuth streamGenerateContent path", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer ya29.fake-token" {
				t.Fatalf("Authorization = %q, want Bearer ya29.fake-token", r.Header.Get("Authorization"))
			}
		})

	for range p.Chat(context.Background(), provider.ChatParams{
		ModelID:  "gemini-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}) {
	}
}

func TestGoogleStreamTextThinkToolCallAndUsage(t *testing.T) {
	sse := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"thinking\",\"thought\":true,\"thoughtSignature\":\"sig-1\"},{\"text\":\"Hello \"}]}}]}\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"read\",\"args\":{\"path\":\"main.go\"}}}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":5,\"thoughtsTokenCount\":2,\"cachedContentTokenCount\":7,\"totalTokenCount\":17}}\n"
	p := newMockGoogleProvider(t,
		NewGeminiProviderWithModels("fake-key", "https://generativelanguage.googleapis.com/v1beta/models", []*provider.Model{{ID: "gemini-test"}}),
		sse,
		nil,
		nil)

	var text string
	var think string
	var thinkSignature string
	var tool *provider.ToolCallBlock
	var usage *provider.Usage
	var done bool
	for ev := range p.Chat(context.Background(), provider.ChatParams{
		ModelID:  "gemini-test",
		Messages: []provider.Message{provider.NewUserMessage("hi")},
		Abort:    make(chan struct{}),
	}) {
		switch ev.Type {
		case provider.StreamTextDelta:
			text += ev.TextDelta
		case provider.StreamThinkDelta:
			think += ev.ThinkDelta
		case provider.StreamThinkSignature:
			thinkSignature = ev.ThinkSignature
		case provider.StreamToolCall:
			tool = ev.ToolCall
		case provider.StreamUsage:
			usage = ev.Usage
		case provider.StreamDone:
			done = true
			if ev.StopReason != "stop" {
				t.Fatalf("stop reason = %q, want stop", ev.StopReason)
			}
		}
	}
	if text != "Hello " {
		t.Fatalf("text = %q, want Hello", text)
	}
	if think != "thinking" {
		t.Fatalf("think = %q, want thinking", think)
	}
	if thinkSignature != "sig-1" {
		t.Fatalf("thinkSignature = %q, want sig-1", thinkSignature)
	}
	if tool == nil || tool.Name != "read" || string(tool.Arguments) != `{"path":"main.go"}` {
		t.Fatalf("tool = %#v, want read path", tool)
	}
	if usage == nil || usage.Input != 10 || usage.Output != 5 || usage.Reasoning != 2 || usage.CacheRead != 7 || usage.TotalTokens != 17 {
		t.Fatalf("usage = %#v, want token counts", usage)
	}
	if !done {
		t.Fatal("missing StreamDone")
	}
}
