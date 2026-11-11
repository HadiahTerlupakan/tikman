-- Whether a graph report tags every member of its group. Off by default: a
-- tag reaches members who muted the group, which a report must not start
-- doing on its own.

ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS mention_all boolean NOT NULL DEFAULT false;
