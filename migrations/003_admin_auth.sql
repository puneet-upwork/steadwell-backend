-- Admin cookie sessions + local bootstrap login (not the system seed user).
CREATE TABLE IF NOT EXISTS admin_sessions (
    id            UUID PRIMARY KEY,
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash    TEXT NOT NULL UNIQUE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO admin_users (id, email, password_hash, display_name, status)
VALUES (
    '2c8e0a11-7b3d-4f2a-9e1c-6d5f4a3b2c1d',
    'admin@steadwell.local',
    '$2b$10$P5OPvnj/e4SPJx4DeqJA8OD00VnMW32zRv1yM2Sqn/ID2ygh5Y8Z2',
    'Admin',
    'active'
)
ON CONFLICT (email) DO NOTHING;
