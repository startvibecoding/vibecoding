package provider

func init() {
	RegisterVendorAdapter(simpleVendorAdapter{
		name:    "groq",
		domains: []string{"api.groq.com"},
	})
}
