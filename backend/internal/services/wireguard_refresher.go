package services

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// reconcileEveryNStatusTicks spaces the convergence reconcile out from the
// status refresh. Apply rewrites the whole peer set and the routing table,
// while a status read only copies counters, so the expensive call runs an order
// of magnitude less often. Drift only appears after an Apply that failed
// part-way, and the worst case — a deleted peer whose key the kernel still
// accepts — is bounded by this cadence rather than by the next restart.
const reconcileEveryNStatusTicks = 10

// RunStatusRefresher keeps the peer status columns current and periodically
// reconciles the device back onto the database. The worker reads those columns
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
			if err := s.Reconcile(); err != nil {
				logger.Warn("Failed to reconcile WireGuard configuration", zap.Error(err))
			}
		}
	}
}
