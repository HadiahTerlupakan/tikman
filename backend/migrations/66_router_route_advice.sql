-- Route advice: the BGP candidates the hourly route read finds for the
-- prefixes holding a router's busiest destinations, and each connection's
-- input filter chain, which the drafted configuration names.
CREATE TABLE IF NOT EXISTS router_prefix_routes (
    router_id  uuid        NOT NULL,
    prefix     varchar(64) NOT NULL,
    routes     text        NOT NULL,
    checked_at timestamptz NOT NULL,
    PRIMARY KEY (router_id, prefix)
);

ALTER TABLE router_prefix_routes DROP CONSTRAINT IF EXISTS router_prefix_routes_router_fk;
ALTER TABLE router_prefix_routes ADD CONSTRAINT router_prefix_routes_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

ALTER TABLE router_bgp_sessions ADD COLUMN IF NOT EXISTS input_filter varchar(255) NOT NULL DEFAULT '';

-- Every existing router gets the new tier, due now; new routers get it from
-- createRouterJobs.
INSERT INTO router_poll_jobs (router_id, kind, due_at)
SELECT id, 'routes', now() FROM mikrotik_routers
ON CONFLICT (router_id, kind) DO NOTHING;
