package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "gitee",
		domains: []string{"ai.gitee.com"},
	})
}
