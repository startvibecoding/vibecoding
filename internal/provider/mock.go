package provider

import "context"

// MockProvider is a mock implementation of Provider for testing.
type MockProvider struct {
	name      string
	models    []*Model
	responses []StreamEvent
	callCount int
}

// NewMockProvider creates a new MockProvider.
func NewMockProvider(name string, models []*Model, responses []StreamEvent) *MockProvider {
	return &MockProvider{
		name:      name,
		models:    models,
		responses: responses,
	}
}

// Chat sends a chat request and returns a channel of streaming events.
func (p *MockProvider) Chat(ctx context.Context, params ChatParams) <-chan StreamEvent {
	ch := make(chan StreamEvent, 100)

	go func() {
		defer close(ch)
		p.callCount++

		for _, event := range p.responses {
			select {
			case <-ctx.Done():
				ch <- StreamEvent{Type: StreamError, Error: ctx.Err()}
				return
			case ch <- event:
			}
		}
	}()

	return ch
}

// Name returns the provider's name.
func (p *MockProvider) Name() string {
	return p.name
}

// Models returns the list of available models.
func (p *MockProvider) Models() []*Model {
	return p.models
}

// GetModel returns a model by ID, or nil if not found.
func (p *MockProvider) GetModel(id string) *Model {
	for _, m := range p.models {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// GetCallCount returns the number of times Chat was called.
func (p *MockProvider) GetCallCount() int {
	return p.callCount
}
