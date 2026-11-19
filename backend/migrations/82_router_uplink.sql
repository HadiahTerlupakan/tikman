-- The interface a BRAS's default route leaves by: the port the faceplate
-- marks upstream and the BRAS page charts. Discovery fills it; deleting the
-- interface clears it until the next discovery finds the new one.
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS uplink_interface_id uuid NULL;
ALTER TABLE mikrotik_routers DROP CONSTRAINT IF EXISTS mikrotik_routers_uplink_interface_fk;
ALTER TABLE mikrotik_routers ADD CONSTRAINT mikrotik_routers_uplink_interface_fk
    FOREIGN KEY (uplink_interface_id) REFERENCES router_interfaces(id) ON DELETE SET NULL;
