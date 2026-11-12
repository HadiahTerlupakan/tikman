-- Port down/up notices to the customer are chosen per port, not per report.
--
-- AutoMigrate runs before this file: it has already added notify_status from
-- the model tag, and no longer knows graph_reports.notify_port_status, so it
-- leaves that column for this file to drop. Each statement still stands on its
-- own against a database built from the SQL files alone.

ALTER TABLE graph_report_ports ADD COLUMN IF NOT EXISTS notify_status boolean NOT NULL DEFAULT false;

-- A report that had notices on keeps them for every port it names. The
-- report-level column is read only where it still exists, so a rerun after the
-- drop below is a no-op.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'graph_reports' AND column_name = 'notify_port_status'
    ) THEN
        UPDATE graph_report_ports p SET notify_status = true
        FROM graph_reports r
        WHERE r.id = p.report_id AND r.notify_port_status;
    END IF;
END $$;

ALTER TABLE graph_reports DROP COLUMN IF EXISTS notify_port_status;
