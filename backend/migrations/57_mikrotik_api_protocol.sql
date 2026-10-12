-- The protocol TikMan reads each MikroTik router with. AutoMigrate adds the
-- column from the model tags; the CHECK is what the tags cannot express.
ALTER TABLE mikrotik_routers
    ADD COLUMN IF NOT EXISTS api_protocol varchar(16) NOT NULL DEFAULT 'rest';

ALTER TABLE mikrotik_routers DROP CONSTRAINT IF EXISTS mikrotik_routers_api_protocol_check;
ALTER TABLE mikrotik_routers ADD CONSTRAINT mikrotik_routers_api_protocol_check
    CHECK (api_protocol IN ('rest', 'api-ssl', 'api'));
