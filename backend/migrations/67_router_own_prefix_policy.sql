-- The router's own announced prefixes, its routing filter rules (where the
-- prepend each peer sees for each prefix is set), each connection's output
-- chain, and per own prefix the bytes over each upstream in five-minute
-- windows, so the UI can show how download for each block comes in.
CREATE TABLE IF NOT EXISTS router_announced_prefixes (
    router_id uuid        NOT NULL,
    prefix    varchar(64) NOT NULL,
    PRIMARY KEY (router_id, prefix)
);
ALTER TABLE router_announced_prefixes DROP CONSTRAINT IF EXISTS router_announced_prefixes_router_fk;
ALTER TABLE router_announced_prefixes ADD CONSTRAINT router_announced_prefixes_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS router_filter_rules (
    router_id uuid         NOT NULL,
    position  integer      NOT NULL,
    chain     varchar(255) NOT NULL,
    rule      text         NOT NULL,
    disabled  boolean      NOT NULL,
    comment   text         NOT NULL,
    PRIMARY KEY (router_id, position)
);
ALTER TABLE router_filter_rules DROP CONSTRAINT IF EXISTS router_filter_rules_router_fk;
ALTER TABLE router_filter_rules ADD CONSTRAINT router_filter_rules_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

ALTER TABLE router_bgp_sessions ADD COLUMN IF NOT EXISTS output_filter varchar(255) NOT NULL DEFAULT '';

-- No foreign keys from the flow table, as for the other flow tables.
CREATE TABLE IF NOT EXISTS router_flow_own_prefix (
    time                  timestamptz NOT NULL,
    router_id             uuid        NOT NULL,
    prefix                varchar(64) NOT NULL,
    upstream_interface_id uuid        NOT NULL,
    direction             varchar(8)  NOT NULL,
    bytes                 bigint      NOT NULL
);
SELECT create_hypertable('router_flow_own_prefix', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_own_prefix', INTERVAL '30 days', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_router_flow_own_prefix_router_time ON router_flow_own_prefix (router_id, time);
