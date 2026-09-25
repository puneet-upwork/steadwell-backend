-- Per-org channel credentials. Secrets are AES-GCM ciphertext; public fields are JSON.
ALTER TABLE organization_channels
    ADD COLUMN IF NOT EXISTS public_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE organization_channels
    ADD COLUMN IF NOT EXISTS credentials_ciphertext BYTEA;

ALTER TABLE organization_channels
    ADD COLUMN IF NOT EXISTS credentials_configured BOOLEAN NOT NULL DEFAULT FALSE;
