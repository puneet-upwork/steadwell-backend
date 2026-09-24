CREATE TABLE IF NOT EXISTS admin_users (
    id            UUID PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT,
    status        TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS organizations (
    id                   UUID PRIMARY KEY,
    slug                 TEXT NOT NULL UNIQUE,
    name                 TEXT NOT NULL,
    legal_name           TEXT,
    status               TEXT NOT NULL DEFAULT 'active',
    join_token           TEXT NOT NULL UNIQUE,
    created_by_admin_id  UUID NOT NULL REFERENCES admin_users(id)
);

CREATE TABLE IF NOT EXISTS channels (
    id     UUID PRIMARY KEY,
    slug   TEXT NOT NULL UNIQUE,
    name   TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS users (
    id               UUID PRIMARY KEY,
    organization_id  UUID NOT NULL REFERENCES organizations(id),
    channel_id       UUID NOT NULL REFERENCES channels(id),
    participant_id   TEXT NOT NULL,
    display_name     TEXT,
    parental_consent BOOLEAN NOT NULL DEFAULT FALSE,
    age_ineligible   BOOLEAN NOT NULL DEFAULT FALSE,
    profile          JSONB NOT NULL DEFAULT '{}'::jsonb,
    status           TEXT NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (channel_id, participant_id)
);

CREATE TABLE IF NOT EXISTS feature_definitions (
    id           UUID PRIMARY KEY,
    key          TEXT NOT NULL UNIQUE,
    description  TEXT,
    implemented  BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS plans (
    id                  UUID PRIMARY KEY,
    slug                TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'active',
    created_by_admin_id UUID NOT NULL REFERENCES admin_users(id)
);

CREATE TABLE IF NOT EXISTS plan_features (
    plan_id    UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    feature_id UUID NOT NULL REFERENCES feature_definitions(id),
    enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    quota      JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (plan_id, feature_id)
);

CREATE TABLE IF NOT EXISTS organization_features (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    feature_id      UUID NOT NULL REFERENCES feature_definitions(id),
    enabled         BOOLEAN NOT NULL DEFAULT FALSE,
    settings        JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (organization_id, feature_id)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id    UUID NOT NULL REFERENCES plans(id),
    status     TEXT NOT NULL DEFAULT 'active',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at    TIMESTAMPTZ,
    UNIQUE (user_id)
);

CREATE TABLE IF NOT EXISTS prompt_modules (
    id          UUID PRIMARY KEY,
    key         TEXT NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE IF NOT EXISTS prompt_versions (
    id           UUID PRIMARY KEY,
    module_id    UUID NOT NULL REFERENCES prompt_modules(id) ON DELETE CASCADE,
    version      TEXT NOT NULL,
    body         TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'published',
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (module_id, version)
);

CREATE TABLE IF NOT EXISTS organization_prompt_stack (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module_id       UUID NOT NULL REFERENCES prompt_modules(id),
    version_id      UUID NOT NULL REFERENCES prompt_versions(id),
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order      INT NOT NULL DEFAULT 0,
    PRIMARY KEY (organization_id, module_id)
);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
