-- The map's "+ Server" button let an OLT be redrawn as an unlinked node with
-- only a name and a position, even though the OLT record already carries its
-- own latitude/longitude (migration 29). Two places to define the same
-- device is exactly what migration 53 removed for ODC/ODP; this migration
-- undoes the same mistake for the OLT itself, by making a server node a
-- mirror of a real OLT rather than a second, independent definition of one.
--
-- mapping_edges still addresses its endpoints by node_id (migration 53), so
-- an OLT still needs a row here for a feeder cable (OLT -> ODC) to point at.
-- olt_id is the link, and it is by id rather than by name: node_id below is
-- derived from the OLT's name once and never recomputed, so a later rename
-- must not orphan the feeders already drawn from it.
--
-- The column and its partial unique index are also declared as GORM tags on
-- models.MappingNode.OLTID, which is what gives the SQLite schema used in
-- unit tests (RunSQLMigrations only runs against Postgres) the same
-- constraint. This file is the explicit, version-tracked schema change
-- CLAUDE.md asks for rather than one left to AutoMigrate alone, and is a
-- no-op if AutoMigrate already put both in place.
ALTER TABLE mapping_nodes ADD COLUMN IF NOT EXISTS olt_id UUID;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mapping_nodes_olt_id
    ON mapping_nodes (olt_id) WHERE olt_id IS NOT NULL;

-- Backfill: three OLTs in production already carry coordinates with nothing
-- on the map to show for it. Only OLTs that already have both latitude and
-- longitude are mirrored - unlike the ODC/ODP backfill in migration 53, an
-- OLT that has never been located has nothing wrong to flag; it simply stays
-- off the map, exactly as it was before this migration.
--
-- Same shape as migration 53's odc_ids/odp_ids: row_number() gives every OLT
-- a clean SERVER-<name> id, and only a genuine collision (two OLTs sharing a
-- name - nothing in this codebase stops that) gets the '-' plus 8 hex
-- character suffix. left(..., 64 - 9) reserves room for that suffix before
-- mapping_nodes.node_id's varchar(64) is ever reached, truncating the
-- prefix+name rather than the finished string for the same reason migration
-- 53 does: truncating after appending the suffix could cut the suffix itself
-- off a long enough name, defeating the very thing it disambiguates. id is a
-- fresh gen_random_uuid() rather than the OLT's own id - unlike ODC/ODP,
-- nothing in this schema already points at "the OLT's map node" by id, so
-- there is no existing reference for a borrowed id to preserve; olt_id is
-- the only link this backfill owes anyone.
--
-- olts.name is varchar(255) but mapping_nodes.name is only varchar(120) - a
-- gap odc_ids/odp_ids never had to close, since odcs.code/odps.code both
-- already fit inside 120. left(name, 120) below is display truncation only:
-- it never touches mapped_node_id, so a long name still disambiguates on its
-- own untruncated collision count.
WITH olt_ids AS (
    SELECT o.*,
           left('SERVER-' || o.name, 64 - 9) || CASE
               WHEN row_number() OVER (PARTITION BY o.name ORDER BY o.created_at, o.id) > 1
               THEN '-' || left(o.id::text, 8) ELSE '' END AS mapped_node_id
    FROM olts o
    WHERE o.latitude IS NOT NULL AND o.longitude IS NOT NULL
)
INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, capacity, olt_id, created_at, updated_at)
SELECT gen_random_uuid(), mapped_node_id, 'server', left(name, 120), latitude, longitude, 0, id, created_at, updated_at
FROM olt_ids
WHERE NOT EXISTS (SELECT 1 FROM mapping_nodes m WHERE m.node_id = olt_ids.mapped_node_id);
