-- How often each top destination address was reached: the sampled flow
-- records its window saw and how many distinct subscribers they came from.
-- Windows written before this read 0 for both.
ALTER TABLE router_flow_top_ips ADD COLUMN IF NOT EXISTS flows bigint NOT NULL DEFAULT 0;
ALTER TABLE router_flow_top_ips ADD COLUMN IF NOT EXISTS subscribers integer NOT NULL DEFAULT 0;
