package gateway

import (
	"net/http"
	"time"
)

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	models := s.provider.Models()
	items := make([]ModelItem, 0, len(models))
	for _, m := range models {
		items = append(items, ModelItem{
			ID:      m.ID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "vibecoding",
		})
	}

	resp := ModelListResponse{
		Object: "list",
		Data:   items,
	}
	writeJSON(w, http.StatusOK, resp)
}
