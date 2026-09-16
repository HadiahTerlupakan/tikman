-- ODC and ODP: the passive plant between a PON port and a subscriber's drop.
--
-- AutoMigrate used to create these tables from the model tags and run before
-- this file, so what was left here was everything a GORM tag cannot say: the
-- rule that an ODP has exactly one parent, and the foreign keys that stop a
-- cabinet or a port from being deleted out from under the boxes hanging off
-- it.
--
-- Task 6 deleted models.ODC/ODCFeed/ODP along with the service built on them,
-- so AutoMigrate no longer creates odcs/odc_feeds/odps at all. Every real
-- database already has them from when it did — schema_migrations marks this
-- version applied there, so the guards below never run again on it. They
-- exist for a schema built from nothing, which is exactly what the test
-- suite's freshPostgres and freshPostgresBeforeMapping do: without a table to
-- add these constraints to, migration 53's backfill would have nothing to
-- read from and migration 54 would have nothing to drop.
CREATE TABLE IF NOT EXISTS odcs (
    id uuid PRIMARY KEY,
    site_id uuid NOT NULL,
    code varchar(64) NOT NULL,
    latitude double precision,
    longitude double precision,
    address text,
    notes text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS odc_feeds (
    id uuid PRIMARY KEY,
    odc_id uuid NOT NULL,
    olt_id uuid NOT NULL,
    slot integer NOT NULL,
    port_id integer NOT NULL,
    splitter_outputs integer NOT NULL,
    notes text,
    route jsonb,
    route_meters double precision NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_odc_feeds_pon UNIQUE (olt_id, slot, port_id)
);

CREATE TABLE IF NOT EXISTS odps (
    id uuid PRIMARY KEY,
    code varchar(64) NOT NULL,
    port_count integer NOT NULL,
    latitude double precision,
    longitude double precision,
    address text,
    notes text,
    route jsonb,
    route_meters double precision NOT NULL DEFAULT 0,
    odc_id uuid,
    olt_id uuid,
    slot integer,
    port_id integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- An ODP hangs off a cabinet or off a PON port, never both and never neither.
-- Networks grow both ways — some feeders reach a cabinet first, some ports go
-- straight to a distribution box — and a row satisfying neither branch would
-- describe a box connected to nothing.
ALTER TABLE odps DROP CONSTRAINT IF EXISTS odps_exactly_one_parent;
ALTER TABLE odps ADD CONSTRAINT odps_exactly_one_parent CHECK (
    (odc_id IS NOT NULL
        AND olt_id IS NULL AND slot IS NULL AND port_id IS NULL)
    OR
    (odc_id IS NULL
        AND olt_id IS NOT NULL AND slot IS NOT NULL AND port_id IS NOT NULL)
);

-- A splitter has outputs; a box with none holds nothing.
ALTER TABLE odps DROP CONSTRAINT IF EXISTS odps_port_count_positive;
ALTER TABLE odps ADD CONSTRAINT odps_port_count_positive CHECK (port_count > 0);

ALTER TABLE odc_feeds DROP CONSTRAINT IF EXISTS odc_feeds_outputs_positive;
ALTER TABLE odc_feeds ADD CONSTRAINT odc_feeds_outputs_positive
    CHECK (splitter_outputs > 0);

-- RESTRICT rather than CASCADE throughout: deleting a cabinet that still feeds
-- distribution boxes, or an OLT that still has plant hanging off it, is a
-- mistake worth refusing rather than a deletion worth spreading.
ALTER TABLE odcs DROP CONSTRAINT IF EXISTS fk_odcs_site;
ALTER TABLE odcs ADD CONSTRAINT fk_odcs_site
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE RESTRICT;

ALTER TABLE odc_feeds DROP CONSTRAINT IF EXISTS fk_odc_feeds_odc;
ALTER TABLE odc_feeds ADD CONSTRAINT fk_odc_feeds_odc
    FOREIGN KEY (odc_id) REFERENCES odcs(id) ON DELETE CASCADE;

ALTER TABLE odc_feeds DROP CONSTRAINT IF EXISTS fk_odc_feeds_olt;
ALTER TABLE odc_feeds ADD CONSTRAINT fk_odc_feeds_olt
    FOREIGN KEY (olt_id) REFERENCES olts(id) ON DELETE RESTRICT;

ALTER TABLE odps DROP CONSTRAINT IF EXISTS fk_odps_odc;
ALTER TABLE odps ADD CONSTRAINT fk_odps_odc
    FOREIGN KEY (odc_id) REFERENCES odcs(id) ON DELETE RESTRICT;

ALTER TABLE odps DROP CONSTRAINT IF EXISTS fk_odps_olt;
ALTER TABLE odps ADD CONSTRAINT fk_odps_olt
    FOREIGN KEY (olt_id) REFERENCES olts(id) ON DELETE RESTRICT;

-- A subscriber losing its ODP row should lose the assignment, not the ONT.
ALTER TABLE onts DROP CONSTRAINT IF EXISTS fk_onts_odp;
ALTER TABLE onts ADD CONSTRAINT fk_onts_odp
    FOREIGN KEY (odp_id) REFERENCES odps(id) ON DELETE SET NULL;

-- A port assignment names both the box and the port on it, or neither.
ALTER TABLE onts DROP CONSTRAINT IF EXISTS onts_odp_assignment_complete;
ALTER TABLE onts ADD CONSTRAINT onts_odp_assignment_complete CHECK (
    (odp_id IS NULL AND odp_port IS NULL)
    OR (odp_id IS NOT NULL AND odp_port IS NOT NULL AND odp_port > 0)
);
