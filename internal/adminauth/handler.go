package adminauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const CookieName = "sw_admin_session"

const sessionTTL = 7 * 24 * time.Hour

type Handler struct {
	pool *pgxpool.Pool
}

func Register(r *gin.Engine, pool *pgxpool.Pool) {
	h := &Handler{pool: pool}
	g := r.Group("/admin/v1")
	g.POST("/auth/login", h.login)
	g.POST("/auth/logout", h.logout)
	g.GET("/auth/me", h.RequireSession, h.me)
}

// Middleware returns cookie session auth for /admin/v1 routes other than login.
func Middleware(pool *pgxpool.Pool) gin.HandlerFunc {
	return (&Handler{pool: pool}).RequireSession
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Admin is the signed-in operator set on the gin context.
type Admin struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

const ContextAdminKey = "admin"

func AdminFrom(c *gin.Context) (Admin, bool) {
	v, ok := c.Get(ContextAdminKey)
	if !ok {
		return Admin{}, false
	}
	admin, ok := v.(Admin)
	return admin, ok
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	var admin Admin
	var hash string
	var status string
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT id::text, email, COALESCE(display_name, ''), password_hash, status
		FROM admin_users WHERE email = $1
	`, email).Scan(&admin.ID, &admin.Email, &admin.DisplayName, &hash, &status)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if status != "active" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	raw, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session"})
		return
	}
	expires := time.Now().Add(sessionTTL)
	_, err = h.pool.Exec(c.Request.Context(), `
		INSERT INTO admin_sessions (id, admin_user_id, token_hash, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)
	`, uuid.NewString(), admin.ID, hashToken(raw), expires)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session"})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	c.JSON(http.StatusOK, admin)
}

func (h *Handler) logout(c *gin.Context) {
	raw, _ := c.Cookie(CookieName)
	if raw != "" {
		_, _ = h.pool.Exec(c.Request.Context(), `DELETE FROM admin_sessions WHERE token_hash = $1`, hashToken(raw))
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) me(c *gin.Context) {
	admin, _ := c.Get(ContextAdminKey)
	c.JSON(http.StatusOK, admin)
}

func (h *Handler) RequireSession(c *gin.Context) {
	raw, err := c.Cookie(CookieName)
	if err != nil || raw == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var admin Admin
	err = h.pool.QueryRow(c.Request.Context(), `
		SELECT u.id::text, u.email, COALESCE(u.display_name, '')
		FROM admin_sessions s
		JOIN admin_users u ON u.id = s.admin_user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW() AND u.status = 'active'
	`, hashToken(raw)).Scan(&admin.ID, &admin.Email, &admin.DisplayName)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Set(ContextAdminKey, admin)
	c.Next()
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
