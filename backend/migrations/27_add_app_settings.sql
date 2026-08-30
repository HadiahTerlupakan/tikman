-- Credentials for external integrations. Values are AES-256-GCM ciphertext
-- written by the API; nothing else writes to this table.
CREATE TABLE IF NOT EXISTS app_settings (
    name VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID
);
