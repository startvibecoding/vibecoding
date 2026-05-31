package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "bailian",
		domains: []string{"dashscope.aliyuncs.com"},
	})
}
