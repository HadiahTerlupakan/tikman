-- Each network's type as its operator states it in PeeringDB, which flowd
-- fetches daily, and whether a top destination address is a Tor exit, a VPN
-- or a hosting network, from free lists flowd checks when a window closes.
CREATE TABLE IF NOT EXISTS asn_types (
    asn        bigint PRIMARY KEY,
    type       text NOT NULL,
    updated_at timestamptz NOT NULL
);

ALTER TABLE router_flow_top_ips ADD COLUMN IF NOT EXISTS anonymity varchar(8) NOT NULL DEFAULT '';
