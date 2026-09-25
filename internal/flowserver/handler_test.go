package flowserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"steadwell/internal/channel"
	"steadwell/internal/flows"
	"steadwell/internal/store"
)

type noopSender struct{}

func (noopSender) Send(context.Context, string, channel.Reply) error { return nil }
func (noopSender) AnswerCallback(context.Context, string, string, bool) error {
	return nil
}

type echoGen struct{}

func (echoGen) Generate(_ context.Context, system string, _ []store.ChatMessage, user string) (string, error) {
	return system + "\n" + user, nil
}

func TestTelegramWebhookLandsOnDefaultOrg(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cat := store.NewSeeded()
	r := gin.New()
	RegisterRoutes(r, &flows.Deps{Catalog: cat, Chat: cat, Telegram: noopSender{}, Generator: echoGen{}}, "")

	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram", strings.NewReader(`{"update_id":1,"message":{"from":{"id":42,"first_name":"Ada"},"chat":{"id":42},"text":"hello"}}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	u, err := cat.LandTelegramUser(req.Context(), "42", "Ada", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.OrganizationSlug != channel.DefaultOrgSlug || u.PlanSlug != channel.DefaultPlanSlug {
		t.Fatalf("user %+v", u)
	}
}
