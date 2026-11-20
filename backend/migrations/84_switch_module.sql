-- Managed switches. AutoMigrate builds the tables from
-- internal/models/switch.go; this adds what tags cannot express: foreign
-- keys, the hypertable and its retention.

ALTER TABLE switches DROP CONSTRAINT IF EXISTS switches_site_fk;
ALTER TABLE switches ADD CONSTRAINT switches_site_fk
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL;

ALTER TABLE switch_interfaces DROP CONSTRAINT IF EXISTS switch_interfaces_switch_fk;
ALTER TABLE switch_interfaces ADD CONSTRAINT switch_interfaces_switch_fk
    FOREIGN KEY (switch_id) REFERENCES switches(id) ON DELETE CASCADE;

ALTER TABLE switch_vlans DROP CONSTRAINT IF EXISTS switch_vlans_switch_fk;
ALTER TABLE switch_vlans ADD CONSTRAINT switch_vlans_switch_fk
    FOREIGN KEY (switch_id) REFERENCES switches(id) ON DELETE CASCADE;

ALTER TABLE switch_neighbors DROP CONSTRAINT IF EXISTS switch_neighbors_switch_fk;
ALTER TABLE switch_neighbors ADD CONSTRAINT switch_neighbors_switch_fk
    FOREIGN KEY (switch_id) REFERENCES switches(id) ON DELETE CASCADE;

ALTER TABLE switch_config_backups DROP CONSTRAINT IF EXISTS switch_config_backups_switch_fk;
ALTER TABLE switch_config_backups ADD CONSTRAINT switch_config_backups_switch_fk
    FOREIGN KEY (switch_id) REFERENCES switches(id) ON DELETE CASCADE;

ALTER TABLE switch_poll_jobs DROP CONSTRAINT IF EXISTS switch_poll_jobs_switch_fk;
ALTER TABLE switch_poll_jobs ADD CONSTRAINT switch_poll_jobs_switch_fk
    FOREIGN KEY (switch_id) REFERENCES switches(id) ON DELETE CASCADE;

-- No foreign key from the metrics to switch_interfaces, for the reason given
-- in 56_mikrotik_monitoring.sql: a cascading delete into compressed chunks is
-- the expensive path, and retention drops an orphaned sample within the week.
SELECT create_hypertable('switch_interface_metrics', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

ALTER TABLE switch_interface_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'interface_id',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('switch_interface_metrics', INTERVAL '2 days', if_not_exists => TRUE);
SELECT add_retention_policy('switch_interface_metrics', INTERVAL '7 days', if_not_exists => TRUE);
