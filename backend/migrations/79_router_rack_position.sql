-- The slot a router sits in, counted from the top of the rack, so the router
-- list can show the rack in the order it is wired.
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS rack_position integer;
