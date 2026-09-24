package store

import "context"

// Cataloger is the org/plan/prompt/user catalog the Telegram flow uses.
type Cataloger interface {
	DefaultOrgID() string
	LandTelegramUser(ctx context.Context, participantID, displayName, joinToken string) (User, error)
	FeatureEnabled(orgID, featureKey string) bool
	SetOrgFeature(orgID, featureKey string, enabled bool)
	PinPrompt(orgID, moduleKey, body string)
	PromptBodies(orgID string) []string
}
