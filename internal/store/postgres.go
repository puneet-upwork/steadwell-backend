package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"steadwell/internal/channel"
)

var _ Cataloger = (*Postgres)(nil)

type Postgres struct {
	pool          *pgxpool.Pool
	defaultOrgID  string
	defaultPlanID string
}

func NewPostgres(ctx context.Context, pool *pgxpool.Pool) (*Postgres, error) {
	if pool == nil {
		return nil, fmt.Errorf("nil pool")
	}
	p := &Postgres{pool: pool}
	err := pool.QueryRow(ctx, `SELECT id FROM organizations WHERE slug = $1`, channel.DefaultOrgSlug).Scan(&p.defaultOrgID)
	if err != nil {
		return nil, fmt.Errorf("default org %q: %w", channel.DefaultOrgSlug, err)
	}
	err = pool.QueryRow(ctx, `SELECT id FROM plans WHERE slug = $1`, channel.DefaultPlanSlug).Scan(&p.defaultPlanID)
	if err != nil {
		return nil, fmt.Errorf("default plan %q: %w", channel.DefaultPlanSlug, err)
	}
	return p, nil
}

func (p *Postgres) DefaultOrgID() string { return p.defaultOrgID }

func (p *Postgres) LandTelegramUser(ctx context.Context, participantID, displayName, joinToken string) (User, error) {
	var u User
	err := p.pool.QueryRow(ctx, `
		SELECT u.id::text, u.organization_id::text, o.slug, p.id::text, p.slug, c.slug, u.participant_id, COALESCE(u.display_name, '')
		FROM users u
		JOIN organizations o ON o.id = u.organization_id
		JOIN channels c ON c.id = u.channel_id
		JOIN subscriptions s ON s.user_id = u.id
		JOIN plans p ON p.id = s.plan_id
		WHERE c.slug = $1 AND u.participant_id = $2
	`, channel.Telegram, participantID).Scan(
		&u.ID, &u.OrganizationID, &u.OrganizationSlug, &u.PlanID, &u.PlanSlug,
		&u.ChannelSlug, &u.ParticipantID, &u.DisplayName,
	)
	if err == nil {
		return u, nil
	}
	if err != pgx.ErrNoRows {
		return User{}, err
	}

	orgID := p.defaultOrgID
	orgSlug := channel.DefaultOrgSlug
	if token := strings.TrimSpace(joinToken); token != "" {
		var id, slug string
		lookupErr := p.pool.QueryRow(ctx, `
			SELECT id::text, slug FROM organizations
			WHERE join_token = $1 AND status <> 'disabled'
		`, token).Scan(&id, &slug)
		if lookupErr == nil {
			orgID = id
			orgSlug = slug
		} else if lookupErr != pgx.ErrNoRows {
			return User{}, lookupErr
		}
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var channelID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM channels WHERE slug = $1`, channel.Telegram).Scan(&channelID); err != nil {
		return User{}, fmt.Errorf("telegram channel: %w", err)
	}

	u = User{
		ID:               uuid.NewString(),
		OrganizationID:   orgID,
		OrganizationSlug: orgSlug,
		PlanID:           p.defaultPlanID,
		PlanSlug:         channel.DefaultPlanSlug,
		ChannelSlug:      channel.Telegram,
		ParticipantID:    participantID,
		DisplayName:      displayName,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, organization_id, channel_id, participant_id, display_name)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
	`, u.ID, u.OrganizationID, channelID, participantID, displayName); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscriptions (id, user_id, plan_id, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'active')
	`, uuid.NewString(), u.ID, u.PlanID); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

func (p *Postgres) FeatureEnabled(orgID, featureKey string) bool {
	var enabled bool
	err := p.pool.QueryRow(context.Background(), `
		SELECT COALESCE(
			(
				SELECT of.enabled
				FROM organization_features of
				WHERE of.organization_id = $1::uuid AND of.feature_id = fd.id
			),
			(
				SELECT pf.enabled
				FROM plan_features pf
				WHERE pf.plan_id = $2::uuid AND pf.feature_id = fd.id
			),
			FALSE
		)
		FROM feature_definitions fd
		WHERE fd.key = $3
	`, orgID, p.defaultPlanID, featureKey).Scan(&enabled)
	if err != nil {
		return false
	}
	return enabled
}

func (p *Postgres) SetOrgFeature(orgID, featureKey string, enabled bool) {
	_, _ = p.pool.Exec(context.Background(), `
		INSERT INTO organization_features (organization_id, feature_id, enabled)
		SELECT $1::uuid, fd.id, $2
		FROM feature_definitions fd
		WHERE fd.key = $3
		ON CONFLICT (organization_id, feature_id) DO UPDATE SET enabled = EXCLUDED.enabled
	`, orgID, enabled, featureKey)
}

func (p *Postgres) PinPrompt(orgID, moduleKey, body string) {
	ctx := context.Background()
	modID := uuid.NewString()
	verID := uuid.NewString()
	_, err := p.pool.Exec(ctx, `
		INSERT INTO prompt_modules (id, key, description)
		VALUES ($1::uuid, $2, $2)
		ON CONFLICT (key) DO NOTHING
	`, modID, moduleKey)
	if err != nil {
		return
	}
	if err := p.pool.QueryRow(ctx, `SELECT id::text FROM prompt_modules WHERE key = $1`, moduleKey).Scan(&modID); err != nil {
		return
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO prompt_versions (id, module_id, version, body, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, 'published')
		ON CONFLICT (module_id, version) DO UPDATE SET body = EXCLUDED.body
	`, verID, modID, uuid.NewString(), body)
	if err != nil {
		return
	}
	if err := p.pool.QueryRow(ctx, `SELECT id::text FROM prompt_versions WHERE module_id = $1::uuid AND body = $2 ORDER BY published_at DESC LIMIT 1`, modID, body).Scan(&verID); err != nil {
		return
	}
	_, _ = p.pool.Exec(ctx, `
		INSERT INTO organization_prompt_stack (organization_id, module_id, version_id, enabled, sort_order)
		VALUES ($1::uuid, $2::uuid, $3::uuid, TRUE, (
			SELECT COALESCE(MAX(sort_order), 0) + 1 FROM organization_prompt_stack WHERE organization_id = $1::uuid
		))
		ON CONFLICT (organization_id, module_id) DO UPDATE SET version_id = EXCLUDED.version_id, enabled = TRUE
	`, orgID, modID, verID)
}

func (p *Postgres) PromptBodies(orgID string) []string {
	rows, err := p.pool.Query(context.Background(), `
		SELECT pv.body
		FROM organization_prompt_stack s
		JOIN prompt_versions pv ON pv.id = s.version_id
		WHERE s.organization_id = $1::uuid AND s.enabled
		ORDER BY s.sort_order
	`, orgID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return out
		}
		out = append(out, body)
	}
	return out
}
