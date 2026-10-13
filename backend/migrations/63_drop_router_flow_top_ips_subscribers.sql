-- Migration 62's subscriber count saw only the POPs' public NAT addresses:
-- the border router exports flows after NAT, so most addresses read one
-- "subscriber" however many people reached them. Counting real subscribers
-- needs flows from the POP routers, before NAT.
ALTER TABLE router_flow_top_ips DROP COLUMN IF EXISTS subscribers;
