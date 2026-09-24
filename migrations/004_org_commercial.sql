-- Org commercial classification + which channels this org's join QR wraps.
-- Category / subcategory / seat_band are admin-facing labels (not plans).
-- organization_channels: enable LINE / WhatsApp / Telegram wrappers for the same join_token.

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'b2c';

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS subcategory TEXT;

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS seat_band TEXT;

CREATE TABLE IF NOT EXISTS organization_channels (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    channel_id      UUID NOT NULL REFERENCES channels(id),
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (organization_id, channel_id)
);

-- Existing orgs: enable Telegram by default (current first channel).
INSERT INTO organization_channels (organization_id, channel_id, enabled)
SELECT o.id, c.id, TRUE
FROM organizations o
CROSS JOIN channels c
WHERE c.slug = 'telegram'
ON CONFLICT (organization_id, channel_id) DO NOTHING;
