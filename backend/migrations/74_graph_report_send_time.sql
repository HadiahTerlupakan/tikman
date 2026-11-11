-- When each graph report starts its daily send, "HH:MM" in WIB. Reports
-- registered before this column keep the 06:00 they were always sent at.

ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS send_time varchar(5) NOT NULL DEFAULT '06:00';
