-- The greeting a graph report's image is sent with. Empty means the default
-- text, so reports registered before this column keep working unchanged.

ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS message text NOT NULL DEFAULT '';
