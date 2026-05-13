package provider

import "context"

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// Chat sends a chat request and returns a channel of streaming events.
	Chat(ctx context.Context, params ChatParams) <-chan StreamEvent

	// Name returns the provider's name (e.g. "openai", "anthropic").
	Name() string

	// Models returns the list of available models.
	Models() []*Model

	// GetModel returns a model by ID, or nil if not found.
	GetModel(id string) *Model
}
