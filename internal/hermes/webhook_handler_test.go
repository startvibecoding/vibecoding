package hermes

import (
	"testing"

	"github.com/startvibecoding/vibecoding/internal/hermes/webhook"
)

func TestWebhookHandlerRequiresMultiAgent(t *testing.T) {
	d := &Dispatcher{agentMgr: nil}
	h := NewWebhookHandler(d, nil)

	route := webhook.RouteConfig{Path: "/test", Skill: "test"}
	err := h.HandleWebhookEvent(nil, route, []byte(`{}`))
	if err == nil {
		t.Error("expected error when agentMgr is nil")
	}
}
