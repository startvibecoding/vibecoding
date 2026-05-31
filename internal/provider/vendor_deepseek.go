package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:           "deepseek",
		domains:        []string{"api.deepseek.com"},
		thinkingFormat: "deepseek",
	})
}
