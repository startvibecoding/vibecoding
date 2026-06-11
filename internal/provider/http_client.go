package provider

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewHTTPClient returns a provider HTTP client. Empty proxyURL preserves the
// default environment proxy behavior from http.Transport.
func NewHTTPClient(timeout time.Duration, proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("proxy URL must include scheme and host")
		}
		transport.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

// ApplyHeaders applies configured custom headers after provider defaults.
func ApplyHeaders(req *http.Request, headers map[string]string) {
	for name, value := range headers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		req.Header.Set(name, value)
	}
}
