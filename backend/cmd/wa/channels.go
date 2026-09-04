package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// syncChannels refreshes one number's channel mirror.
//
// A failure leaves the previous list in place rather than emptying the picker:
// Replace is not reached at all when WhatsApp does not answer, so a number
// whose connection is briefly down keeps the channels it had.
func syncChannels(
	ctx context.Context,
	client *wa.Client,
	channels *services.CSChannelService,
	accountID uuid.UUID,
	logger *zap.Logger,
) {
	found, err := client.AdminChannels(ctx)
	if err != nil {
		logger.Warn("Could not read the channel list", zap.Error(err))
		return
	}
	if err := channels.Replace(accountID, found); err != nil {
		logger.Error("Could not store the channel list", zap.Error(err))
		return
	}
	logger.Info("Refreshed the channel list", zap.Int("channels", len(found)))
}
