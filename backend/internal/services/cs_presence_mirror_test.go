package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeSnapshot struct {
	ids []uuid.UUID
	err error
}

func (f *fakeSnapshot) Present(context.Context) ([]uuid.UUID, error) {
	return f.ids, f.err
}

func TestMirrorWritesEveryAgentTheSnapshotHolds(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	presence := NewFakePresence()
	mirror := NewPresenceMirror(&fakeSnapshot{ids: []uuid.UUID{first, second}}, presence, zap.NewNop())

	require.NoError(t, mirror.Sync(context.Background()))

	online, err := presence.Online(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{first, second}, online)
}

// The point of the whole migration. Letting a departed agent's key expire
// instead would leave assignment as slow as the sixty-second TTL it replaces.
func TestMirrorDeletesAnAgentTheSnapshotNoLongerHolds(t *testing.T) {
	staying, leaving := uuid.New(), uuid.New()
	presence := NewFakePresence()
	snapshot := &fakeSnapshot{ids: []uuid.UUID{staying, leaving}}
	mirror := NewPresenceMirror(snapshot, presence, zap.NewNop())
	require.NoError(t, mirror.Sync(context.Background()))

	snapshot.ids = []uuid.UUID{staying}
	require.NoError(t, mirror.Sync(context.Background()))

	online, err := presence.Online(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{staying}, online)
}

// One failed read is a blip, not an evacuation. Emptying the rotation on it
// would stop assignment for everybody over a single dropped request; the TTL
// is what handles a real outage, by expiring keys nobody refreshes.
func TestMirrorLeavesTheSetAloneWhenASnapshotFails(t *testing.T) {
	agent := uuid.New()
	presence := NewFakePresence()
	snapshot := &fakeSnapshot{ids: []uuid.UUID{agent}}
	mirror := NewPresenceMirror(snapshot, presence, zap.NewNop())
	require.NoError(t, mirror.Sync(context.Background()))

	snapshot.err = errors.New("rtdb unreachable")
	require.Error(t, mirror.Sync(context.Background()))

	online, err := presence.Online(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{agent}, online)
}
