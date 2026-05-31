package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "minimax",
		domains: []string{"api.minimax.chat"},
	})
}
