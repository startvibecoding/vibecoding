package gateway

import "net/http"

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	resp := HealthResponse{
		Status:   "ok",
		Version:  s.version,
		Sessions: s.pool.Count(),
	}
	writeJSON(w, http.StatusOK, resp)
}
