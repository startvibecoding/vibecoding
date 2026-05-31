package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:       "openai",
		domains:    []string{"api.openai.com"},
		defaultAPI: "openai-chat",
	})
}
