package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

var errDown = errors.New("database unavailable")

// fakeSink fails the next calls with the errors queued for them, then stores.
type fakeSink struct {
	minuteErrs, topErrs, ownErrs []error
	minutes                      [][]netflow.MinuteRow
	tops                         [][]netflow.TopIPRow
	owns                         [][]netflow.OwnPrefixRow
}

func next(errs *[]error) error {
	if len(*errs) == 0 {
		return nil
	}
	err := (*errs)[0]
	*errs = (*errs)[1:]
	return err
}

func (s *fakeSink) WriteMinute(_ context.Context, rows []netflow.MinuteRow) error {
	if err := next(&s.minuteErrs); err != nil {
		return err
	}
	s.minutes = append(s.minutes, rows)
	return nil
}

func (s *fakeSink) WriteTopIPs(_ context.Context, rows []netflow.TopIPRow) error {
	if err := next(&s.topErrs); err != nil {
		return err
	}
	s.tops = append(s.tops, rows)
	return nil
}

func (s *fakeSink) WriteOwnPrefixes(_ context.Context, rows []netflow.OwnPrefixRow) error {
	if err := next(&s.ownErrs); err != nil {
		return err
	}
	s.owns = append(s.owns, rows)
	return nil
}

func (s *fakeSink) MarkSeen(context.Context, []uuid.UUID, time.Time) error { return nil }

func minuteOf(bytes uint64) pendingBatch {
	return pendingBatch{minute: []netflow.MinuteRow{{Bytes: bytes}}}
}

func TestFlushRetriesAFailedBatchWithTheNext(t *testing.T) {
	sink := &fakeSink{minuteErrs: []error{errDown}}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}

	w.flush(minuteOf(1))
	w.flush(minuteOf(2))

	assert.Equal(t, [][]netflow.MinuteRow{{{Bytes: 1}}, {{Bytes: 2}}}, sink.minutes)
	assert.Nil(t, w.pending)
}

// Review Focus 4: a database that stays down costs one batch per failed
// retry, never a growing backlog.
func TestFlushDropsABatchThatFailsTwice(t *testing.T) {
	sink := &fakeSink{minuteErrs: []error{errDown, errDown, errDown}}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}

	w.flush(minuteOf(1)) // fails, held
	w.flush(minuteOf(2)) // the retry of 1 fails: dropped; 2 fails: held
	w.flush(minuteOf(3)) // 2 and 3 are written

	assert.Equal(t, [][]netflow.MinuteRow{{{Bytes: 2}}, {{Bytes: 3}}}, sink.minutes)
}

// Review Focus 4: the minute was stored and only the top-IP write failed;
// the retry must not store that minute a second time.
func TestRetryDoesNotStoreAMinuteTwice(t *testing.T) {
	sink := &fakeSink{topErrs: []error{errDown}}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}

	w.flush(pendingBatch{minute: []netflow.MinuteRow{{Bytes: 1}}, topIPs: []netflow.TopIPRow{{Bytes: 9}}})
	w.flush(minuteOf(2))

	assert.Equal(t, [][]netflow.MinuteRow{{{Bytes: 1}}, {{Bytes: 2}}}, sink.minutes)
	assert.Equal(t, [][]netflow.TopIPRow{{{Bytes: 9}}}, sink.tops)
}

// The top addresses went in and only the own-prefix write failed: the retry
// writes the own prefixes alone.
func TestFlushRetriesOnlyTheOwnPrefixesThatFailed(t *testing.T) {
	sink := &fakeSink{ownErrs: []error{errDown}}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}
	batch := pendingBatch{topIPs: []netflow.TopIPRow{{Bytes: 7}}, ownPrefixes: []netflow.OwnPrefixRow{{Bytes: 9}}}

	w.flush(batch)
	w.flush(pendingBatch{})

	assert.Equal(t, [][]netflow.TopIPRow{{{Bytes: 7}}}, sink.tops)
	assert.Equal(t, [][]netflow.OwnPrefixRow{{{Bytes: 9}}}, sink.owns)
	assert.Nil(t, w.pending)
}
