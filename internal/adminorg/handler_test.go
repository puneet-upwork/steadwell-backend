package adminorg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"steadwell/internal/adminauth"
	"steadwell/internal/dbmigrate"
	"steadwell/internal/joinlinks"
)

func TestOrganizationsRequireAuth(t *testing.T) {
	r := testRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/v1/organizations", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}

func TestCreateListUpdateSoftDeleteAndQR(t *testing.T) {
	r := testRouter(t)
	cookie := loginCookie(t, r)
	suffix := uniqueSuffix()
	name := fmt.Sprintf("Acme Care %d", suffix)
	wantSlug := fmt.Sprintf("acme-care-%d", suffix)

	create := doJSON(t, r, http.MethodPost, "/admin/v1/organizations", cookie,
		fmt.Sprintf(`{"name":%q,"status":"active","category":"b2b","subcategory":"b1_corporate_wellness","seat_band":"501-2000","channels":{"telegram":true,"line":true,"whatsapp":false}}`, name))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status %d body %s", create.Code, create.Body.String())
	}
	var org map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &org); err != nil {
		t.Fatal(err)
	}
	id, _ := org["id"].(string)
	slug, _ := org["slug"].(string)
	if id == "" || slug != wantSlug || org["category"] != "b2b" || org["seat_band"] != "501-2000" {
		t.Fatalf("create body %s", create.Body.String())
	}
	channelsRaw, _ := org["channels"].([]any)
	if len(channelsRaw) == 0 {
		t.Fatalf("expected channels on create: %s", create.Body.String())
	}
	var telegramJoin string
	for _, raw := range channelsRaw {
		ch, _ := raw.(map[string]any)
		if ch["slug"] == "telegram" && ch["enabled"] == true {
			telegramJoin, _ = ch["join_url"].(string)
		}
	}
	if !strings.Contains(telegramJoin, "t.me/SteadwellTestBot?start=") {
		t.Fatalf("expected telegram channel join_url, got channels %s", create.Body.String())
	}
	if _, ok := org["user_count"]; !ok {
		t.Fatalf("expected user_count in create body %s", create.Body.String())
	}
	if _, hasJoinURL := org["join_url"]; hasJoinURL {
		t.Fatalf("org-level join_url should be removed: %s", create.Body.String())
	}

	list := doJSON(t, r, http.MethodGet, "/admin/v1/organizations", cookie, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), slug) {
		t.Fatalf("list missing slug: %s", list.Body.String())
	}

	patch := doJSON(t, r, http.MethodPatch, "/admin/v1/organizations/"+id, cookie,
		`{"name":"Acme Care Updated","legal_name":"Acme Care Ltd","status":"draft"}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch status %d body %s", patch.Code, patch.Body.String())
	}
	if !strings.Contains(patch.Body.String(), "Acme Care Updated") || !strings.Contains(patch.Body.String(), `"draft"`) {
		t.Fatalf("patch body %s", patch.Body.String())
	}

	oldToken, _ := org["join_token"].(string)
	rot := doJSON(t, r, http.MethodPost, "/admin/v1/organizations/"+id+"/join-token/rotate", cookie, "")
	if rot.Code != http.StatusOK {
		t.Fatalf("rotate status %d body %s", rot.Code, rot.Body.String())
	}
	var rotated map[string]any
	if err := json.Unmarshal(rot.Body.Bytes(), &rotated); err != nil {
		t.Fatal(err)
	}
	if rotated["join_token"] == oldToken || rotated["join_token"] == nil {
		t.Fatalf("rotate did not change token: %s", rot.Body.String())
	}

	qr := doJSON(t, r, http.MethodGet, "/admin/v1/organizations/"+id+"/qr", cookie, "")
	if qr.Code != http.StatusOK {
		t.Fatalf("qr status %d body %s", qr.Code, qr.Body.String())
	}
	var qrPayload map[string]any
	if err := json.Unmarshal(qr.Body.Bytes(), &qrPayload); err != nil {
		t.Fatal(err)
	}
	if qrPayload["join_token"] != rotated["join_token"] {
		t.Fatalf("qr body %s", qr.Body.String())
	}

	del := doJSON(t, r, http.MethodDelete, "/admin/v1/organizations/"+id, cookie, "")
	if del.Code != http.StatusOK {
		t.Fatalf("delete status %d body %s", del.Code, del.Body.String())
	}

	list2 := doJSON(t, r, http.MethodGet, "/admin/v1/organizations", cookie, "")
	if strings.Contains(list2.Body.String(), slug) {
		t.Fatalf("soft-deleted org still listed: %s", list2.Body.String())
	}
}

func testRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testPool(t)
	if err := dbmigrate.Apply(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := gin.New()
	adminauth.Register(r, pool)
	Register(r, pool, joinlinks.Config{
		TelegramBotUsername: "SteadwellTestBot",
		WhatsAppNumber:      "15551234567",
		LineLiffURL:         "https://liff.line.me/test-liff",
	})
	return r
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://127.0.0.1:5432/steadwell?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("local postgres not up: %v", err)
	}
	return pool
}

func loginCookie(t *testing.T, r *gin.Engine) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(
		`{"email":"admin@steadwell.local","password":"steadwell"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d body %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == adminauth.CookieName {
			return c.Value
		}
	}
	t.Fatal("missing session cookie")
	return ""
}

func doJSON(t *testing.T, r *gin.Engine, method, path, cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Cookie", adminauth.CookieName+"="+cookie)
	r.ServeHTTP(w, req)
	return w
}

func uniqueSuffix() int64 {
	return time.Now().UnixNano() % 1_000_000_000
}
