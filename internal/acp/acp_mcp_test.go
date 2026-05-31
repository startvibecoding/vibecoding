package acp

import (
	"encoding/json"
	"testing"
)

func TestExtractSamplingInput(t *testing.T) {
	raw := json.RawMessage(`{"maxTokens":512,"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}]}`)
	prompt, systemPrompt, maxTokens := extractSamplingInput(raw)
	if prompt != "hello" {
		t.Errorf("prompt: got %q", prompt)
	}
	if systemPrompt != "sys" {
		t.Errorf("systemPrompt: got %q", systemPrompt)
	}
	if maxTokens != 512 {
		t.Errorf("maxTokens: got %d", maxTokens)
	}
}

func TestParseJSONRawToMap(t *testing.T) {
	raw := json.RawMessage("{}")
	m := parseJSONRawToMap(raw)
	if m == nil {
		t.Fatal("expected map")
	}
	m = parseJSONRawToMap(json.RawMessage("bad"))
	if m != nil {
		t.Error("expected nil")
	}
}
