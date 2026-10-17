package main

import (
	"context"

	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// syncGroups refreshes one number's group mirror. A failure leaves the last
// list in place.
func syncGroups(ctx context.Context, client *wa.Client, logger *zap.Logger) {
	count, err := client.SyncGroups(ctx)
	if err != nil {
		logger.Warn("Could not read the group list", zap.Error(err))
		return
	}
	logger.Info("Refreshed the group list", zap.Int("groups", count))
}

// drainGroups sends one number's waiting group messages and carries out its
// waiting management requests.
func drainGroups(ctx context.Context, live *session, logger *zap.Logger) {
	if sent, err := live.groupDrainer.Drain(ctx, outboxBatch); err != nil {
		logger.Error("Could not send the queued group messages", zap.Error(err))
	} else if sent > 0 {
		logger.Info("Sent queued group messages", zap.Int("count", sent))
	}
	if done, err := live.groupActions.Run(ctx, outboxBatch); err != nil {
		logger.Error("Could not carry out the queued group actions", zap.Error(err))
	} else if done > 0 {
		logger.Info("Carried out queued group actions", zap.Int("count", done))
	}
}
