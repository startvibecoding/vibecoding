package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "kimi",
		domains: []string{"api.moonshot.cn"},
	})
}
