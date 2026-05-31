package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "seed",
		domains: []string{"ark.cn-beijing.volces.com"},
	})
}
