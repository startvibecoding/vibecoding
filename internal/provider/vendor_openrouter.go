package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "openrouter",
		domains: []string{"openrouter.ai"},
	})
}
