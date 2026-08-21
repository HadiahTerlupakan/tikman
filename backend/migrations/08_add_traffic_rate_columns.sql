-- Add real-time traffic rate columns from CLI telnet
ALTER TABLE ont_metrics ADD COLUMN IF NOT EXISTS rx_rate_mbps DOUBLE PRECISION;
ALTER TABLE ont_metrics ADD COLUMN IF NOT EXISTS tx_rate_mbps DOUBLE PRECISION;

-- Add indexes for rate queries
CREATE INDEX IF NOT EXISTS idx_ont_metrics_rx_rate ON ont_metrics(rx_rate_mbps) WHERE rx_rate_mbps IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ont_metrics_tx_rate ON ont_metrics(tx_rate_mbps) WHERE tx_rate_mbps IS NOT NULL;
