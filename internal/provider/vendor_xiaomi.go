package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:           "xiaomi-token-plan-ams",
		domains:        []string{"token-plan-ams.xiaomimimo.com"},
		thinkingFormat: "xiaomi",
	})
	RegisterVendorAdapter(simpleVendorAdapter{
		name:           "xiaomi-token-plan-cn",
		domains:        []string{"token-plan-cn.xiaomimimo.com"},
		thinkingFormat: "xiaomi",
	})
	RegisterVendorAdapter(simpleVendorAdapter{
		name:           "xiaomi-token-plan-sgp",
		domains:        []string{"token-plan-sgp.xiaomimimo.com"},
		thinkingFormat: "xiaomi",
	})
	RegisterVendorAdapter(simpleVendorAdapter{
		name:           "xiaomi",
		domains:        []string{"api.xiaomimimo.com", "api.xiaomi.com"},
		thinkingFormat: "xiaomi",
	})
}
