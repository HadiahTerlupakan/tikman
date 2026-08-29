package services

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// RunStatusRefresher keeps the peer status columns current. The worker reads
// those columns instead of the kernel, so only this process needs privileges.
func (s *WireGuardService) RunStatusRefresher(ctx context.Context, interval time.Duration, logger *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RefreshStatus(); err != nil {
				logger.Warn("Failed to refresh WireGuard status", zap.Error(err))
			}
		}
	}
}
