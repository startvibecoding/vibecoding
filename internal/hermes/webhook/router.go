// Package webhook implements inbound webhook routing for Hermes mode.
// External services (GitHub, CI, etc.) POST events to /webhook/<path>,
// which are verified and dispatched to agent tasks.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

// RouteConfig defines a webhook route.
type RouteConfig struct {
	Path     string   `json:"path"`
	Events   []string `json:"events"`
	Skill    string   `json:"skill"`
	Delivery string   `json:"delivery"` // "wechat", "feishu", or "" (no delivery)
}

// Handler processes incoming webhook events.
type Handler interface {
	HandleWebhookEvent(ctx context.Context, route RouteConfig, payload []byte) error
}

// Router manages webhook routes and dispatches events.
type Router struct {
	routes  []RouteConfig
	secret  string
	handler Handler
}

// NewRouter creates a webhook router.
func NewRouter(routes []RouteConfig, secret string, handler Handler) *Router {
	return &Router{
		routes:  routes,
		secret:  secret,
		handler: handler,
	}
}

// ServeHTTP handles incoming webhook requests.
// Expected path: /webhook/<route-path>
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract the route path from URL
	path := strings.TrimPrefix(req.URL.Path, "/webhook")
	if path == "" {
		path = "/"
	}

	// Find matching route
	var route *RouteConfig
	for i := range r.routes {
		if r.routes[i].Path == path {
			route = &r.routes[i]
			break
		}
	}
	if route == nil {
		http.Error(w, "no route for path: "+path, http.StatusNotFound)
		return
	}

	// Read body
	body, err := io.ReadAll(io.LimitReader(req.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		http.Error(w, "read body error", http.StatusBadRequest)
		return
	}

	// Verify signature if secret is configured
	if r.secret != "" {
		sig := req.Header.Get("X-Hub-Signature-256")
		if sig == "" {
			sig = req.Header.Get("X-Signature-256")
		}
		if !r.verifySignature(body, sig) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Check event type filter
	eventType := req.Header.Get("X-GitHub-Event")
	if eventType == "" {
		// Try to extract from body
		var generic struct {
			Action string `json:"action"`
			Type   string `json:"type"`
		}
		json.Unmarshal(body, &generic)
		if generic.Action != "" {
			eventType = generic.Action
		} else if generic.Type != "" {
			eventType = generic.Type
		}
	}

	if len(route.Events) > 0 && eventType != "" {
		matched := false
		for _, ev := range route.Events {
			if ev == eventType || ev == "*" {
				matched = true
				break
			}
		}
		if !matched {
			// Event type not in filter — acknowledge but skip
			writeJSON(w, http.StatusOK, map[string]string{"status": "skipped", "reason": "event type not matched"})
			return
		}
	}

	// Dispatch to handler
	log.Printf("[webhook] Received event on %s (type: %s, %d bytes)", path, eventType, len(body))

	if r.handler != nil {
		go func() {
			if err := r.handler.HandleWebhookEvent(context.Background(), *route, body); err != nil {
				log.Printf("[webhook] Handler error for %s: %v", path, err)
			}
		}()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// verifySignature verifies HMAC-SHA256 signature.
func (r *Router) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// Strip "sha256=" prefix
	sig := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(r.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expected))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Ensure Router satisfies http.Handler.
var _ http.Handler = (*Router)(nil)
