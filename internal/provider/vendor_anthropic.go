package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:       "anthropic",
		domains:    []string{"api.anthropic.com"},
		defaultAPI: "anthropic-messages",
	})
	RegisterVendorAdapter(simpleVendorAdapter{
		name:       "claude",
		domains:    []string{},
		defaultAPI: "anthropic-messages",
	})
}
