-- router_flow_top_ips.country now comes from IPinfo Lite instead of DB-IP
-- (migration 61): on the addresses that carry most traffic here, latency
-- measured from the VPS agreed with IPinfo about twice as often. Rows DB-IP
-- located are cleared so a range does not mix the two; the UI falls back to
-- the AS's registered country until IPinfo has located them again.
UPDATE router_flow_top_ips SET country = '' WHERE country <> '';
