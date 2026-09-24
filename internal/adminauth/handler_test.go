package adminauth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"steadwell/internal/dbmigrate"
)

func TestLoginRejectsBadPassword(t *testing.T) {
	r := testRouter(t)
	w := httptest.NewRecorder()
	body := `{"email":"admin@steadwell.local","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestLoginSetsCookieThenMeAndLogout(t *testing.T) {
	r := testRouter(t)

	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(
		`{"email":"admin@steadwell.local","password":"steadwell"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login status %d body %s", login.Code, login.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(login.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["email"] != "admin@steadwell.local" {
		t.Fatalf("login body %s", login.Body.String())
	}
	cookie := sessionCookie(login)
	if cookie == "" {
		t.Fatal("expected session cookie")
	}

	me := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/admin/v1/auth/me", nil)
	meReq.Header.Set("Cookie", CookieName+"="+cookie)
	r.ServeHTTP(me, meReq)
	if me.Code != http.StatusOK {
		t.Fatalf("me status %d body %s", me.Code, me.Body.String())
	}

	out := httptest.NewRecorder()
	outReq := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/logout", bytes.NewReader(nil))
	outReq.Header.Set("Cookie", CookieName+"="+cookie)
	r.ServeHTTP(out, outReq)
	if out.Code != http.StatusOK {
		t.Fatalf("logout status %d", out.Code)
	}

	me2 := httptest.NewRecorder()
	me2Req := httptest.NewRequest(http.MethodGet, "/admin/v1/auth/me", nil)
	me2Req.Header.Set("Cookie", CookieName+"="+cookie)
	r.ServeHTTP(me2, me2Req)
	if me2.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status %d body %s", me2.Code, me2.Body.String())
	}
}

func TestMeWithoutCookieUnauthorized(t *testing.T) {
	r := testRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/v1/auth/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
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

func sessionCookie(w *httptest.ResponseRecorder) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			return c.Value
		}
	}
	return ""
}
