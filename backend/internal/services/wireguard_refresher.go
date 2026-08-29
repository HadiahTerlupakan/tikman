package services

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// reconcileEveryNStatusTicks spaces the pending-reconcile check out from the
// status refresh. The check itself is cheap, but the retry it may run replaces
// the whole peer set, so a site that is still handshaking should not be exposed
// to it on every status tick. It bounds how long a drift left by a failed Apply
// can outlive the database — the worst case being a deleted peer whose key the
// kernel still accepts — rather than leaving it until the next restart.
const reconcileEveryNStatusTicks = 10

// RunStatusRefresher keeps the peer status columns current and retries a
// reconcile that an earlier Apply left pending. The worker reads those columns
// instead of the kernel, so only this process needs privileges.
func (s *WireGuardService) RunStatusRefresher(ctx context.Context, interval time.Duration, logger *zap.Logger) {
	status := time.NewTicker(interval)
	defer status.Stop()
	converge := time.NewTicker(interval * reconcileEveryNStatusTicks)
	defer converge.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-status.C:
			if err := s.RefreshStatus(); err != nil {
				logger.Warn("Failed to refresh WireGuard status", zap.Error(err))
			}
		case <-converge.C:
			if err := s.ReconcileIfPending(); err != nil {
				logger.Warn("Failed to reconcile WireGuard configuration", zap.Error(err))
			}
		}
	}
}
