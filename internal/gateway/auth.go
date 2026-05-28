package gateway

import (
	"net/http"
	"strings"
)

// AuthMiddleware returns an HTTP middleware that validates Bearer tokens.
// If auth is disabled, the handler is called directly.
func AuthMiddleware(cfg AuthConfig, next http.Handler) http.Handler {
	if !cfg.Enabled || len(cfg.Tokens) == 0 {
		return next
	}
	tokenSet := make(map[string]struct{}, len(cfg.Tokens))
	for _, t := range cfg.Tokens {
		tokenSet[t] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header", "authentication_error")
			return
		}
		if _, ok := tokenSet[token]; !ok {
			writeError(w, http.StatusUnauthorized, "invalid API key", "authentication_error")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware adds CORS headers when enabled.
func CORSMiddleware(cfg CORSConfig, next http.Handler) http.Handler {
	if !cfg.Enabled {
		return next
	}
	origins := "*"
	if len(cfg.AllowOrigins) > 0 {
		origins = strings.Join(cfg.AllowOrigins, ", ")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ConcurrencyMiddleware limits the number of concurrent in-flight requests.
// If maxConcurrent <= 0, no limit is applied.
func ConcurrencyMiddleware(maxConcurrent int, next http.Handler) http.Handler {
	if maxConcurrent <= 0 {
		return next
	}
	sem := make(chan struct{}, maxConcurrent)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		default:
			writeError(w, http.StatusTooManyRequests, "server is at capacity, please retry later", "rate_limit_error")
		}
	})
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}
