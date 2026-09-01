-- A site can terminate more than one tunnel. Cariu is one site with two POPs,
-- each behind its own router and its own WireGuard peer, and the uniqueness on
-- site_id made the second one impossible to register.
--
-- Nothing is lost by dropping it. The invariant that actually matters is that
-- two tunnels must not claim the same subnet, and rejectOverlapWithPeers in
-- wireguard_validate.go has always enforced that independently. site_id
-- uniqueness only ever restated a guess about topology.
--
-- Two shapes are dropped because two exist. AutoMigrate runs before this file
-- and builds the uniqueness as an INDEX from the model tag, which is what
-- production actually holds; a database built from 25_add_wireguard.sql instead
-- holds it as a table CONSTRAINT. Dropping only one of them leaves the other
-- standing, and the failure is silent.
DROP INDEX IF EXISTS idx_wireguard_peers_site_id;

ALTER TABLE wireguard_peers DROP CONSTRAINT IF EXISTS wireguard_peers_site_id_key;

-- Recreated non-unique under the name AutoMigrate expects, so it recognises the
-- index as present and does not restore the unique one. The worker reads a
-- site's peers every poll cycle to decide which OLTs sit behind a tunnel that
-- is down.
CREATE INDEX IF NOT EXISTS idx_wireguard_peers_site_id ON wireguard_peers(site_id);
