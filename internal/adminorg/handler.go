package adminorg

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"steadwell/internal/adminauth"
	"steadwell/internal/joinlinks"
)

const StatusDisabled = "disabled"
const StatusActive = "active"
const StatusDraft = "draft"

var slugRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var allowedStatuses = map[string]bool{
	StatusActive: true,
	StatusDraft:  true,
}

var allowedCategories = map[string]bool{
	"b2c": true,
	"b2b": true,
	"b2g": true,
}

var allowedSeatBands = map[string]bool{
	"":            true,
	"1-500":       true,
	"501-2000":    true,
	"2000-plus":   true,
	"custom":      true,
}

type Handler struct {
	pool     *pgxpool.Pool
	joinCfg  joinlinks.Config
}

func Register(r *gin.Engine, pool *pgxpool.Pool, joinCfg joinlinks.Config) {
	h := &Handler{pool: pool, joinCfg: joinCfg}
	g := r.Group("/admin/v1")
	g.Use(adminauth.Middleware(pool))
	g.GET("/features", h.listFeatures)
	g.GET("/channels", h.listChannels)
	g.GET("/organizations", h.list)
	g.POST("/organizations", h.create)
	g.GET("/organizations/:id", h.get)
	g.PATCH("/organizations/:id", h.update)
	g.DELETE("/organizations/:id", h.softDelete)
	g.GET("/organizations/:id/qr", h.qr)
	g.POST("/organizations/:id/join-token/rotate", h.rotateJoinToken)
	g.PUT("/organizations/:id/features", h.putFeatures)
	g.PUT("/organizations/:id/channels", h.putChannels)
}

type featureDefJSON struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled,omitempty"`
}

type channelJSON struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled,omitempty"`
	JoinURL string `json:"join_url,omitempty"`
}

type orgJSON struct {
	ID           string           `json:"id"`
	Slug         string           `json:"slug"`
	Name         string           `json:"name"`
	LegalName    *string          `json:"legal_name"`
	Status       string           `json:"status"`
	Category     string           `json:"category"`
	Subcategory  *string          `json:"subcategory"`
	SeatBand     *string          `json:"seat_band"`
	JoinToken    string           `json:"join_token"`
	CreatedBy    string           `json:"created_by_admin_id"`
	UserCount    int              `json:"user_count"`
	Features     []featureDefJSON `json:"features,omitempty"`
	Channels     []channelJSON    `json:"channels,omitempty"`
}

type createRequest struct {
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	LegalName   *string         `json:"legal_name"`
	Status      string          `json:"status"`
	Category    string          `json:"category"`
	Subcategory *string         `json:"subcategory"`
	SeatBand    *string         `json:"seat_band"`
	Features    map[string]bool `json:"features"`
	Channels    map[string]bool `json:"channels"`
}

type updateRequest struct {
	Name        *string `json:"name"`
	LegalName   *string `json:"legal_name"`
	Status      *string `json:"status"`
	Category    *string `json:"category"`
	Subcategory *string `json:"subcategory"`
	SeatBand    *string `json:"seat_band"`
}

type putFeaturesRequest struct {
	Features map[string]bool `json:"features"`
}

type putChannelsRequest struct {
	Channels map[string]bool `json:"channels"`
}

func slugFromName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastHyphen := false
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "org"
	}
	if len(s) > 48 {
		s = strings.Trim(s[:48], "-")
	}
	if !slugRE.MatchString(s) {
		return "org"
	}
	return s
}

func optionalTrim(p *string) any {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return v
}

func (h *Handler) channelJoinURL(slug, token string) string {
	return joinlinks.ForChannel(h.joinCfg, slug, token)
}

func (h *Handler) listFeatures(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT key, COALESCE(description, '')
		FROM feature_definitions
		WHERE implemented
		ORDER BY key
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	defer rows.Close()
	out := make([]featureDefJSON, 0)
	for rows.Next() {
		var f featureDefJSON
		if err := rows.Scan(&f.Key, &f.Description); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
			return
		}
		out = append(out, f)
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) listChannels(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT slug, name FROM channels WHERE status = 'active' ORDER BY slug
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	defer rows.Close()
	out := make([]channelJSON, 0)
	for rows.Next() {
		var ch channelJSON
		if err := rows.Scan(&ch.Slug, &ch.Name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
			return
		}
		out = append(out, ch)
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) list(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	statusFilter := strings.TrimSpace(c.Query("status"))
	categoryFilter := strings.ToLower(strings.TrimSpace(c.Query("category")))

	sql := `
		SELECT o.id::text, o.slug, o.name, o.legal_name, o.status, o.category,
			o.subcategory, o.seat_band, o.join_token, o.created_by_admin_id::text,
			(SELECT COUNT(*)::int FROM users u WHERE u.organization_id = o.id)
		FROM organizations o
		WHERE o.status <> $1`
	args := []any{StatusDisabled}
	argN := 2
	if statusFilter != "" && allowedStatuses[statusFilter] {
		sql += fmt.Sprintf(` AND o.status = $%d`, argN)
		args = append(args, statusFilter)
		argN++
	}
	if categoryFilter != "" && allowedCategories[categoryFilter] {
		sql += fmt.Sprintf(` AND o.category = $%d`, argN)
		args = append(args, categoryFilter)
		argN++
	}
	if q != "" {
		sql += fmt.Sprintf(` AND (o.name ILIKE $%d OR o.slug ILIKE $%d OR COALESCE(o.legal_name, '') ILIKE $%d)`, argN, argN, argN)
		args = append(args, "%"+q+"%")
	}
	sql += ` ORDER BY o.name ASC`

	rows, err := h.pool.Query(c.Request.Context(), sql, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	defer rows.Close()

	out := make([]orgJSON, 0)
	for rows.Next() {
		org, err := scanOrgList(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
			return
		}
		out = append(out, org)
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) get(c *gin.Context) {
	org, err := h.getByID(c, c.Param("id"))
	if err != nil || org.Status == StatusDisabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) create(c *gin.Context) {
	admin, ok := adminauth.AdminFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	name := strings.TrimSpace(req.Name)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = StatusActive
	}
	category := strings.ToLower(strings.TrimSpace(req.Category))
	if category == "" {
		category = "b2c"
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	if !allowedStatuses[status] || !allowedCategories[category] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status or category"})
		return
	}
	seatBand := ""
	if req.SeatBand != nil {
		seatBand = strings.TrimSpace(*req.SeatBand)
	}
	if !allowedSeatBands[seatBand] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seat_band"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = slugFromName(name)
	}
	if !slugRE.MatchString(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slug or name"})
		return
	}

	id := uuid.NewString()
	token := uuid.NewString()
	baseSlug := slug
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", baseSlug, attempt+1)
		}
		_, err = h.pool.Exec(c.Request.Context(), `
			INSERT INTO organizations (
				id, slug, name, legal_name, status, category, subcategory, seat_band,
				join_token, created_by_admin_id
			) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10::uuid)
		`, id, slug, name, optionalTrim(req.LegalName), status, category,
			optionalTrim(req.Subcategory), nullIfEmpty(seatBand), token, admin.ID)
		if err == nil {
			break
		}
		if !isUniqueViolation(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
			return
		}
		if strings.TrimSpace(req.Slug) != "" {
			c.JSON(http.StatusConflict, gin.H{"error": "slug already exists"})
			return
		}
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "slug already exists"})
		return
	}

	features := req.Features
	if features == nil {
		features = map[string]bool{"text_messages": true}
	}
	if err := h.writeFeatures(c, id, features); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "features failed"})
		return
	}
	channels := req.Channels
	if channels == nil {
		channels = map[string]bool{"telegram": true}
	}
	if err := h.writeChannels(c, id, channels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "channels failed"})
		return
	}

	org, err := h.getByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	c.JSON(http.StatusCreated, org)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	org, err := h.getByID(c, id)
	if err != nil || org.Status == StatusDisabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	name := org.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
			return
		}
	}
	status := org.Status
	if req.Status != nil {
		status = strings.TrimSpace(*req.Status)
		if !allowedStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
	}
	category := org.Category
	if req.Category != nil {
		category = strings.ToLower(strings.TrimSpace(*req.Category))
		if !allowedCategories[category] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
			return
		}
	}

	var legal any
	if req.LegalName != nil {
		legal = optionalTrim(req.LegalName)
	} else if org.LegalName != nil {
		legal = *org.LegalName
	}

	var subcategory any
	if req.Subcategory != nil {
		subcategory = optionalTrim(req.Subcategory)
	} else if org.Subcategory != nil {
		subcategory = *org.Subcategory
	}

	var seatBand any
	if req.SeatBand != nil {
		sb := strings.TrimSpace(*req.SeatBand)
		if !allowedSeatBands[sb] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seat_band"})
			return
		}
		seatBand = nullIfEmpty(sb)
	} else if org.SeatBand != nil {
		seatBand = *org.SeatBand
	}

	_, err = h.pool.Exec(c.Request.Context(), `
		UPDATE organizations
		SET name = $2, legal_name = $3, status = $4, category = $5, subcategory = $6, seat_band = $7
		WHERE id = $1::uuid AND status <> $8
	`, id, name, legal, status, category, subcategory, seatBand, StatusDisabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	org, err = h.getByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) softDelete(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE organizations SET status = $2
		WHERE id = $1::uuid AND status <> $2
	`, id, StatusDisabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET status = $2
		WHERE organization_id = $1::uuid AND status <> $2
	`, id, StatusDisabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) qr(c *gin.Context) {
	org, err := h.getByID(c, c.Param("id"))
	if err != nil || org.Status == StatusDisabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         org.ID,
		"slug":       org.Slug,
		"name":       org.Name,
		"join_token": org.JoinToken,
		"channels":   org.Channels,
	})
}

func (h *Handler) rotateJoinToken(c *gin.Context) {
	id := c.Param("id")
	token := uuid.NewString()
	tag, err := h.pool.Exec(c.Request.Context(), `
		UPDATE organizations SET join_token = $2
		WHERE id = $1::uuid AND status <> $3
	`, id, token, StatusDisabled)
	if err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "token collision"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rotate failed"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	org, err := h.getByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rotate failed"})
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) putFeatures(c *gin.Context) {
	id := c.Param("id")
	org, err := h.getByID(c, id)
	if err != nil || org.Status == StatusDisabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req putFeaturesRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Features == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if err := h.writeFeatures(c, id, req.Features); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "features failed"})
		return
	}
	org, err = h.getByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "features failed"})
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) putChannels(c *gin.Context) {
	id := c.Param("id")
	org, err := h.getByID(c, id)
	if err != nil || org.Status == StatusDisabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req putChannelsRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Channels == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if err := h.writeChannels(c, id, req.Channels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "channels failed"})
		return
	}
	org, err = h.getByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "channels failed"})
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) writeFeatures(c *gin.Context, orgID string, features map[string]bool) error {
	rows, err := h.pool.Query(c.Request.Context(), `SELECT id::text, key FROM feature_definitions WHERE implemented`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return err
		}
		_, err := h.pool.Exec(c.Request.Context(), `
			INSERT INTO organization_features (organization_id, feature_id, enabled)
			VALUES ($1::uuid, $2::uuid, $3)
			ON CONFLICT (organization_id, feature_id) DO UPDATE SET enabled = EXCLUDED.enabled
		`, orgID, id, features[key])
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) writeChannels(c *gin.Context, orgID string, channels map[string]bool) error {
	rows, err := h.pool.Query(c.Request.Context(), `SELECT id::text, slug FROM channels WHERE status = 'active'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, slug string
		if err := rows.Scan(&id, &slug); err != nil {
			return err
		}
		_, err := h.pool.Exec(c.Request.Context(), `
			INSERT INTO organization_channels (organization_id, channel_id, enabled)
			VALUES ($1::uuid, $2::uuid, $3)
			ON CONFLICT (organization_id, channel_id) DO UPDATE SET enabled = EXCLUDED.enabled
		`, orgID, id, channels[slug])
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) loadFeatures(c *gin.Context, orgID string) ([]featureDefJSON, error) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT fd.key, COALESCE(fd.description, ''), COALESCE(of.enabled, FALSE)
		FROM feature_definitions fd
		LEFT JOIN organization_features of
			ON of.feature_id = fd.id AND of.organization_id = $1::uuid
		WHERE fd.implemented
		ORDER BY fd.key
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]featureDefJSON, 0)
	for rows.Next() {
		var f featureDefJSON
		if err := rows.Scan(&f.Key, &f.Description, &f.Enabled); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func (h *Handler) loadChannels(c *gin.Context, orgID, joinToken string) ([]channelJSON, error) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT c.slug, c.name, COALESCE(oc.enabled, FALSE)
		FROM channels c
		LEFT JOIN organization_channels oc
			ON oc.channel_id = c.id AND oc.organization_id = $1::uuid
		WHERE c.status = 'active'
		ORDER BY c.slug
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]channelJSON, 0)
	for rows.Next() {
		var ch channelJSON
		if err := rows.Scan(&ch.Slug, &ch.Name, &ch.Enabled); err != nil {
			return nil, err
		}
		ch.JoinURL = h.channelJoinURL(ch.Slug, joinToken)
		out = append(out, ch)
	}
	return out, nil
}

func (h *Handler) getByID(c *gin.Context, id string) (orgJSON, error) {
	row := h.pool.QueryRow(c.Request.Context(), `
		SELECT o.id::text, o.slug, o.name, o.legal_name, o.status, o.category,
			o.subcategory, o.seat_band, o.join_token, o.created_by_admin_id::text,
			(SELECT COUNT(*)::int FROM users u WHERE u.organization_id = o.id)
		FROM organizations o WHERE o.id = $1::uuid
	`, id)
	org, err := scanOrgList(row)
	if err != nil {
		return orgJSON{}, err
	}
	feats, err := h.loadFeatures(c, id)
	if err != nil {
		return orgJSON{}, err
	}
	chs, err := h.loadChannels(c, id, org.JoinToken)
	if err != nil {
		return orgJSON{}, err
	}
	org.Features = feats
	org.Channels = chs
	return org, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanOrgList(row scannable) (orgJSON, error) {
	var org orgJSON
	var legal, subcategory, seatBand *string
	err := row.Scan(
		&org.ID, &org.Slug, &org.Name, &legal, &org.Status, &org.Category,
		&subcategory, &seatBand, &org.JoinToken, &org.CreatedBy, &org.UserCount,
	)
	if err != nil {
		return orgJSON{}, err
	}
	org.LegalName = legal
	org.Subcategory = subcategory
	org.SeatBand = seatBand
	return org, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}
