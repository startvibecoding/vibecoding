package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "fireworks",
		domains: []string{"api.fireworks.ai"},
	})
}
