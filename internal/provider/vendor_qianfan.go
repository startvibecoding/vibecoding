package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "qianfan",
		domains: []string{"aip.baidubce.com"},
	})
}
