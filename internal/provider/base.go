package provider

// BaseProvider provides common functionality for provider implementations.
type BaseProvider struct {
	name   string
	models []*Model
}

// NewBaseProvider creates a new BaseProvider.
func NewBaseProvider(name string, models []*Model) BaseProvider {
	return BaseProvider{name: name, models: models}
}

// Name returns the provider's name.
func (p *BaseProvider) Name() string {
	return p.name
}

// Models returns the list of available models.
func (p *BaseProvider) Models() []*Model {
	return p.models
}

// GetModel returns a model by ID, or nil if not found.
func (p *BaseProvider) GetModel(id string) *Model {
	for _, m := range p.models {
		if m.ID == id {
			return m
		}
	}
	return nil
}
