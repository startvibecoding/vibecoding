package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "together",
		domains: []string{"api.together.xyz"},
	})
}
