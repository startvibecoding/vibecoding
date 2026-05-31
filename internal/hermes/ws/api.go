package ws

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// --- HTTP REST API handlers ---

// handleHealth returns server health status (no auth required).
func (gw *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"version":        gw.version,
		"uptime_seconds": int(time.Since(gw.startTime).Seconds()),
	})
}

// handleStatus returns detailed server status.
func (gw *Gateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gw.mu.RLock()
	dispatcher := gw.dispatcher
	platformProvider := gw.platforms
	gw.mu.RUnlock()

	sessionCount := 0
	if dispatcher != nil {
		sessionCount = len(dispatcher.ListSessions())
	}

	var platforms []PlatformStatus
	if platformProvider != nil {
		platforms = platformProvider.GetPlatformStatuses()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":        gw.version,
		"uptime_seconds": int(time.Since(gw.startTime).Seconds()),
		"sessions": map[string]int{
			"active":      sessionCount,
			"connections": gw.ConnectionCount(),
		},
		"platforms": platforms,
	})
}

// handleSessions lists or manages sessions.
func (gw *Gateway) handleSessions(w http.ResponseWriter, r *http.Request) {
	gw.mu.RLock()
	dispatcher := gw.dispatcher
	gw.mu.RUnlock()

	if dispatcher == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dispatcher not ready"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		sessions := dispatcher.ListSessions()
		writeJSON(w, http.StatusOK, map[string]any{
			"sessions": sessions,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionByID handles GET/DELETE for a specific session.
func (gw *Gateway) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	gw.mu.RLock()
	dispatcher := gw.dispatcher
	gw.mu.RUnlock()

	if dispatcher == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dispatcher not ready"})
		return
	}

	// Extract session ID from path: /api/sessions/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session ID required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		sessions := dispatcher.ListSessions()
		for _, s := range sessions {
			if s.ID == path {
				writeJSON(w, http.StatusOK, s)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})

	case http.MethodDelete:
		dispatcher.RemoveSession(path)
		writeJSON(w, http.StatusOK, map[string]any{
			"message": "session deleted",
			"id":      path,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMemory handles memory.md read/write.
func (gw *Gateway) handleMemory(w http.ResponseWriter, r *http.Request) {
	gw.mu.RLock()
	memStore := gw.memoryStore
	gw.mu.RUnlock()

	if memStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "memory store not configured"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		content, path, source, err := memStore.Read()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"path":    path,
			"source":  source,
			"content": content,
		})

	case http.MethodPut:
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if err := memStore.WriteAll(body.Content); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "memory updated"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlatforms returns messaging platform statuses.
func (gw *Gateway) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gw.mu.RLock()
	platformProvider := gw.platforms
	gw.mu.RUnlock()

	var platforms []PlatformStatus
	if platformProvider != nil {
		platforms = platformProvider.GetPlatformStatuses()
	}
	if platforms == nil {
		platforms = []PlatformStatus{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"platforms": platforms,
	})
}
