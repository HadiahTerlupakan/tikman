CREATE TABLE IF NOT EXISTS wireguard_server (
    id UUID PRIMARY KEY,
    interface_name VARCHAR(15) NOT NULL DEFAULT 'wg0' UNIQUE,
    listen_port INTEGER NOT NULL DEFAULT 51820,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    endpoint_host VARCHAR(255) NOT NULL,
    tunnel_subnet VARCHAR(45) NOT NULL DEFAULT '10.88.0.0/24',
    address VARCHAR(45) NOT NULL DEFAULT '10.88.0.1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wireguard_peers (
    id UUID PRIMARY KEY,
    site_id UUID NOT NULL UNIQUE REFERENCES sites(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    public_key TEXT NOT NULL,
    private_key TEXT NOT NULL,
    preshared_key TEXT,
    tunnel_address VARCHAR(45) NOT NULL UNIQUE,
    allowed_ips JSONB NOT NULL,
    persistent_keepalive INTEGER NOT NULL DEFAULT 25,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_handshake_at TIMESTAMPTZ,
    rx_bytes BIGINT NOT NULL DEFAULT 0,
    tx_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wireguard_peers_enabled ON wireguard_peers(enabled);
