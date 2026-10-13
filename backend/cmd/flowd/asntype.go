package main

import (
	"context"
	"net/http"
	"time"

	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// asnTypesInterval is how often PeeringDB is asked; network types change
// rarely, and the anonymous API is rate limited.
const asnTypesInterval = 24 * time.Hour

// asnTypesRetry is how soon a failed fetch is tried again. PeeringDB lets an
// anonymous client fetch the whole network list about once an hour and
// answers 429 until then; asking sooner only keeps it throttled.
const asnTypesRetry = time.Hour

type asnTypeSaver interface {
	SaveASNTypes(ctx context.Context, types map[uint32]string, updatedAt time.Time) error
}

// refreshASNTypes fetches PeeringDB's network types into the database and
// reports whether it succeeded. On failure the types already saved stay.
func refreshASNTypes(client *http.Client, url string, saver asnTypeSaver, logger *zap.Logger) bool {
	types, err := netflow.FetchASNTypes(client, url)
	if err != nil {
		logger.Warn("ASN type download failed; keeping the types saved", zap.Error(err))
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), asnSaveTimeout)
	defer cancel()
	if err := saver.SaveASNTypes(ctx, types, time.Now()); err != nil {
		logger.Error("Failed to save ASN types", zap.Error(err))
		return false
	}
	logger.Info("ASN types loaded", zap.Int("networks", len(types)))
	return true
}

// keepASNTypes refreshes daily, and an hour after a failure.
func keepASNTypes(client *http.Client, url string, saver asnTypeSaver, logger *zap.Logger) {
	for {
		if refreshASNTypes(client, url, saver, logger) {
			time.Sleep(asnTypesInterval)
		} else {
			time.Sleep(asnTypesRetry)
		}
	}
}
