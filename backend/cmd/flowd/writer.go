package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// flowWriteTimeout bounds one write. A retry and the new minute must both
// finish inside the minute, or the next close waits behind them.
const flowWriteTimeout = 20 * time.Second

// flowSink is where closed minutes go: services.FlowStore in production.
type flowSink interface {
	WriteMinute(ctx context.Context, rows []netflow.MinuteRow) error
	WriteTopIPs(ctx context.Context, rows []netflow.TopIPRow) error
	MarkSeen(ctx context.Context, routerIDs []uuid.UUID, at time.Time) error
}

// pendingBatch is what one minute close produced.
type pendingBatch struct {
	minute []netflow.MinuteRow
	topIPs []netflow.TopIPRow
}

// retryWriter writes each batch and keeps the unwritten part of one that
// failed for a single retry with the next. A batch failing twice is dropped:
// holding batches for a database that stays slow is how a collector runs
// out of memory.
type retryWriter struct {
	sink    flowSink
	logger  *zap.Logger
	pending *pendingBatch
}

func (w *retryWriter) flush(batch pendingBatch) {
	if w.pending != nil {
		if _, err := w.write(*w.pending); err != nil {
			w.logger.Error("Dropping a flow batch that failed twice", zap.Error(err),
				zap.Int("minute_rows", len(w.pending.minute)), zap.Int("top_ip_rows", len(w.pending.topIPs)))
		}
		w.pending = nil
	}
	if rest, err := w.write(batch); err != nil {
		w.logger.Warn("Flow write failed; retrying with the next minute", zap.Error(err))
		w.pending = &rest
	}
}

// write stores what it can and returns what is still unwritten, so a retry
// never stores a minute that already went in.
func (w *retryWriter) write(b pendingBatch) (pendingBatch, error) {
	ctx, cancel := context.WithTimeout(context.Background(), flowWriteTimeout)
	defer cancel()
	if len(b.minute) > 0 {
		if err := w.sink.WriteMinute(ctx, b.minute); err != nil {
			return b, err
		}
		b.minute = nil
	}
	if len(b.topIPs) > 0 {
		if err := w.sink.WriteTopIPs(ctx, b.topIPs); err != nil {
			return b, err
		}
	}
	return pendingBatch{}, nil
}
