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
		fmt.Sprintf(`{"name":%q,"status":"active","category":"b2b","subcategory":"b1_corporate_wellness","seat_band":"501-2000","channels":{"telegram":{"enabled":true,"bot_username":"SteadwellTestBot","bot_token":"tok","webhook_secret":"sec"},"line":{"enabled":true,"liff_url":"https://liff.line.me/test-liff","channel_secret":"ls","channel_access_token":"lat"},"whatsapp":{"enabled":false}}}`, name))
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
	if strings.Contains(create.Body.String(), "bot_token") || strings.Contains(create.Body.String(), `"tok"`) {
		t.Fatalf("create must not return secrets: %s", create.Body.String())
	}
	for _, raw := range channelsRaw {
		ch, _ := raw.(map[string]any)
		if ch["slug"] == "telegram" && ch["configured"] != true {
			t.Fatalf("telegram should be configured: %s", create.Body.String())
		}
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

func TestCreateRejectsEnabledChannelWithoutCreds(t *testing.T) {
	r := testRouter(t)
	cookie := loginCookie(t, r)
	name := fmt.Sprintf("No Creds %d", uniqueSuffix())
	create := doJSON(t, r, http.MethodPost, "/admin/v1/organizations", cookie,
		fmt.Sprintf(`{"name":%q,"channels":{"telegram":{"enabled":true,"bot_username":"x"}}}`, name))
	if create.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", create.Code, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), "bot_token") {
		t.Fatalf("expected bot_token error, got %s", create.Body.String())
	}
}

func TestPutChannelsKeepExistingSecrets(t *testing.T) {
	r := testRouter(t)
	cookie := loginCookie(t, r)
	name := fmt.Sprintf("Keep Creds %d", uniqueSuffix())
	create := doJSON(t, r, http.MethodPost, "/admin/v1/organizations", cookie,
		fmt.Sprintf(`{"name":%q,"channels":{"telegram":{"enabled":true,"bot_username":"firstbot","bot_token":"tok","webhook_secret":"sec"}}}`, name))
	if create.Code != http.StatusCreated {
		t.Fatalf("create %d %s", create.Code, create.Body.String())
	}
	var org map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &org); err != nil {
		t.Fatal(err)
	}
	id, _ := org["id"].(string)
	put := doJSON(t, r, http.MethodPut, "/admin/v1/organizations/"+id+"/channels", cookie,
		`{"channels":{"telegram":{"enabled":true,"bot_username":"secondbot"}}}`)
	if put.Code != http.StatusOK {
		t.Fatalf("put %d %s", put.Code, put.Body.String())
	}
	if !strings.Contains(put.Body.String(), "secondbot") {
		t.Fatalf("username not updated: %s", put.Body.String())
	}
	if strings.Contains(put.Body.String(), "bot_token") {
		t.Fatalf("secrets leaked: %s", put.Body.String())
	}
	got := doJSON(t, r, http.MethodGet, "/admin/v1/organizations/"+id, cookie, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get %d %s", got.Code, got.Body.String())
	}
	if strings.Contains(got.Body.String(), "bot_token") || strings.Contains(got.Body.String(), `"tok"`) {
		t.Fatalf("GET org must not return secrets: %s", got.Body.String())
	}
	doJSON(t, r, http.MethodDelete, "/admin/v1/organizations/"+id, cookie, "")
}

func TestDeleteOrganizationDisablesUsers(t *testing.T) {
	r := testRouter(t)
	cookie := loginCookie(t, r)
	pool := testPool(t)
	suffix := uniqueSuffix()
	name := fmt.Sprintf("Disable Users Org %d", suffix)

	create := doJSON(t, r, http.MethodPost, "/admin/v1/organizations", cookie,
		fmt.Sprintf(`{"name":%q,"status":"active"}`, name))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status %d body %s", create.Code, create.Body.String())
	}
	var org map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &org); err != nil {
		t.Fatal(err)
	}
	id, _ := org["id"].(string)

	ctx := context.Background()
	var channelID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM channels WHERE slug = 'telegram'`).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, organization_id, channel_id, participant_id, display_name, status)
		VALUES (gen_random_uuid(), $1::uuid, $2::uuid, $3, 'DelTest', 'active')
	`, id, channelID, fmt.Sprintf("tg-%d", suffix))
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	del := doJSON(t, r, http.MethodDelete, "/admin/v1/organizations/"+id, cookie, "")
	if del.Code != http.StatusOK {
		t.Fatalf("delete status %d body %s", del.Code, del.Body.String())
	}

	var userStatus string
	if err := pool.QueryRow(ctx, `
		SELECT status FROM users WHERE organization_id = $1::uuid LIMIT 1
	`, id).Scan(&userStatus); err != nil {
		t.Fatal(err)
	}
	if userStatus != "disabled" {
		t.Fatalf("user status = %q, want disabled", userStatus)
	}
}

func testRouter(t *testing.T) *gin.Engine {
	t.Helper()
	t.Setenv("CHANNEL_CREDS_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	gin.SetMode(gin.TestMode)
	pool := testPool(t)
	if err := dbmigrate.Apply(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := gin.New()
	adminauth.Register(r, pool)
	Register(r, pool)
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
