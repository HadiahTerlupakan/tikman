-- Daily LibreNMS port graphs sent into a registered WhatsApp group.
--
-- AutoMigrate creates both tables from the model tags before this file runs;
-- the columns are restated here so the versioned files say what the schema
-- is, and the constraints are what a tag cannot say.

CREATE TABLE IF NOT EXISTS graph_reports (
    id uuid PRIMARY KEY,
    wa_group_id uuid NOT NULL,
    customer_name varchar(255) NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    last_sent_on date,
    last_attempt_at timestamptz,
    last_error text NOT NULL DEFAULT '',
    created_by uuid,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE TABLE IF NOT EXISTS graph_report_ports (
    id uuid PRIMARY KEY,
    report_id uuid NOT NULL,
    device_id integer NOT NULL,
    device_name varchar(255) NOT NULL,
    if_name varchar(255) NOT NULL,
    label varchar(255) NOT NULL,
    position integer NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_reports_wa_group_id ON graph_reports (wa_group_id);
CREATE INDEX IF NOT EXISTS idx_graph_report_ports_report_id ON graph_report_ports (report_id);

-- A report goes with its group, and its ports with the report.
ALTER TABLE graph_reports DROP CONSTRAINT IF EXISTS fk_graph_reports_group;
ALTER TABLE graph_reports ADD CONSTRAINT fk_graph_reports_group
    FOREIGN KEY (wa_group_id) REFERENCES wa_groups(id) ON DELETE CASCADE;

ALTER TABLE graph_report_ports DROP CONSTRAINT IF EXISTS fk_graph_report_ports_report;
ALTER TABLE graph_report_ports ADD CONSTRAINT fk_graph_report_ports_report
    FOREIGN KEY (report_id) REFERENCES graph_reports(id) ON DELETE CASCADE;

ALTER TABLE graph_reports DROP CONSTRAINT IF EXISTS fk_graph_reports_created_by;
ALTER TABLE graph_reports ADD CONSTRAINT fk_graph_reports_created_by
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;
