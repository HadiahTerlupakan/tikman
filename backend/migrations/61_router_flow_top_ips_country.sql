-- Where each top destination address is located, from DB-IP's country
-- dataset, so the UI can show which country the traffic actually went to.
-- The AS's registered country (asn_names.country) is not that: a provider
-- registered in one country runs servers in many.
ALTER TABLE router_flow_top_ips ADD COLUMN IF NOT EXISTS country varchar(2) NOT NULL DEFAULT '';
