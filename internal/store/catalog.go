package store

import (
	"context"
	_ "embed"
	"sync"

	"github.com/google/uuid"

	"steadwell/internal/channel"
)

// PromptKey is the only module seeded and whitelisted on org steadwell.
const PromptKey = "prompt"

//go:embed telegram_prompt.txt
var defaultPromptBody string

type User struct {
	ID               string
	OrganizationID   string
	OrganizationSlug string
	PlanID           string
	PlanSlug         string
	ChannelSlug      string
	ParticipantID    string
	DisplayName      string
}

type Catalog struct {
	mu sync.Mutex

	defaultOrgID  string
	defaultPlanID string

	orgs     map[string]org
	plans    map[string]plan
	features map[string]string // key -> id
	channels map[string]string // slug -> id

	planFeatures map[string]bool // planID|featureID
	orgFeatures  map[string]bool // orgID|featureID -> enabled (present means override)

	users         map[string]User   // channelSlug|participantID
	subscriptions map[string]string // userID -> planID

	promptModules  map[string]promptModule
	promptVersions map[string]promptVersion
	promptStack    []stackEntry

	chat map[string][]memChatMsg // channel|chatID -> turns
}

type org struct {
	ID        string
	Slug      string
	Name      string
	JoinToken string
}

type plan struct {
	ID   string
	Slug string
	Name string
}

type promptModule struct {
	ID   string
	Key  string
	Name string
}

type promptVersion struct {
	ID       string
	ModuleID string
	Body     string
}

type stackEntry struct {
	OrgID     string
	ModuleID  string
	VersionID string
	Enabled   bool
	Order     int
}

var _ Cataloger = (*Catalog)(nil)

func NewSeeded() *Catalog {
	c := &Catalog{
		orgs:           map[string]org{},
		plans:          map[string]plan{},
		features:       map[string]string{},
		channels:       map[string]string{},
		planFeatures:   map[string]bool{},
		orgFeatures:    map[string]bool{},
		users:          map[string]User{},
		subscriptions:  map[string]string{},
		promptModules:  map[string]promptModule{},
		promptVersions: map[string]promptVersion{},
	}

	c.channels[channel.Telegram] = uuid.NewString()
	c.channels[channel.WhatsApp] = uuid.NewString()
	c.channels[channel.Line] = uuid.NewString()

	for _, key := range []string{channel.FeatureText, channel.FeatureImage, channel.FeatureAudio, channel.FeatureVideo} {
		c.features[key] = uuid.NewString()
	}

	c.defaultOrgID = uuid.NewString()
	c.orgs[c.defaultOrgID] = org{ID: c.defaultOrgID, Slug: channel.DefaultOrgSlug, Name: "Steadwell"}

	c.defaultPlanID = uuid.NewString()
	c.plans[c.defaultPlanID] = plan{ID: c.defaultPlanID, Slug: channel.DefaultPlanSlug, Name: "Free"}
	c.planFeatures[c.defaultPlanID+"|"+c.features[channel.FeatureText]] = true

	modID := uuid.NewString()
	verID := uuid.NewString()
	c.promptModules[PromptKey] = promptModule{ID: modID, Key: PromptKey, Name: "prompt"}
	c.promptVersions[verID] = promptVersion{ID: verID, ModuleID: modID, Body: defaultPromptBody}
	c.promptStack = append(c.promptStack, stackEntry{
		OrgID: c.defaultOrgID, ModuleID: modID, VersionID: verID, Enabled: true, Order: 1,
	})
	return c
}

func (c *Catalog) DefaultOrgID() string { return c.defaultOrgID }

func (c *Catalog) LandTelegramUser(_ context.Context, participantID, displayName, joinToken string) (User, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := channel.Telegram + "|" + participantID
	if u, ok := c.users[key]; ok {
		return u, nil
	}
	orgID := c.defaultOrgID
	orgSlug := channel.DefaultOrgSlug
	if joinToken != "" {
		if o, ok := c.orgByJoinToken(joinToken); ok {
			orgID = o.ID
			orgSlug = o.Slug
		}
	}
	u := User{
		ID:               uuid.NewString(),
		OrganizationID:   orgID,
		OrganizationSlug: orgSlug,
		PlanID:           c.defaultPlanID,
		PlanSlug:         channel.DefaultPlanSlug,
		ChannelSlug:      channel.Telegram,
		ParticipantID:    participantID,
		DisplayName:      displayName,
	}
	c.users[key] = u
	c.subscriptions[u.ID] = c.defaultPlanID
	return u, nil
}

func (c *Catalog) orgByJoinToken(token string) (org, bool) {
	for _, o := range c.orgs {
		if o.JoinToken == token {
			return o, true
		}
	}
	return org{}, false
}

func (c *Catalog) SetOrgFeature(orgID, featureKey string, enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fid, ok := c.features[featureKey]
	if !ok {
		return
	}
	c.orgFeatures[orgID+"|"+fid] = enabled
}

func (c *Catalog) FeatureEnabled(orgID, featureKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	fid, ok := c.features[featureKey]
	if !ok {
		return false
	}
	if enabled, overridden := c.orgFeatures[orgID+"|"+fid]; overridden {
		return enabled
	}
	org, ok := c.orgs[orgID]
	if !ok {
		return false
	}
	planID := c.defaultPlanID
	if org.ID != c.defaultOrgID {
		planID = c.defaultPlanID
	}
	return c.planFeatures[planID+"|"+fid]
}

func (c *Catalog) PinPrompt(orgID, moduleKey, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	mod, ok := c.promptModules[moduleKey]
	if !ok {
		mod = promptModule{ID: uuid.NewString(), Key: moduleKey, Name: moduleKey}
		c.promptModules[moduleKey] = mod
	}
	verID := uuid.NewString()
	c.promptVersions[verID] = promptVersion{ID: verID, ModuleID: mod.ID, Body: body}
	c.promptStack = append(c.promptStack, stackEntry{
		OrgID: orgID, ModuleID: mod.ID, VersionID: verID, Enabled: true, Order: len(c.promptStack) + 1,
	})
}

func (c *Catalog) PromptBodies(orgID string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, e := range c.promptStack {
		if e.OrgID != orgID || !e.Enabled {
			continue
		}
		v, ok := c.promptVersions[e.VersionID]
		if !ok {
			continue
		}
		out = append(out, v.Body)
	}
	return out
}

func FeatureKeyForMedia(kind string) string {
	switch kind {
	case channel.MediaImage:
		return channel.FeatureImage
	case channel.MediaAudio:
		return channel.FeatureAudio
	case channel.MediaVideo:
		return channel.FeatureVideo
	default:
		return channel.FeatureText
	}
}

func UnsupportedMediaMessage(kind string) string {
	return "We did not support image messages."
}
