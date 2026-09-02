package services

import (
	"context"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fake is what every assignment test runs against, so its own behaviour is
// worth pinning: turns must advance, or a round-robin test could pass while
// handing every conversation to the same person.
func TestFakePresenceAdvancesItsTurn(t *testing.T) {
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	p := NewFakePresence(a, b)

	first, err := p.NextTurn(ctx)
	require.NoError(t, err)
	second, err := p.NextTurn(ctx)
	require.NoError(t, err)

	assert.Equal(t, first+1, second)

	// Order, not just membership: the rotation walks this slice by index, so a
	// list that came back in a different order each call would hand work out at
	// random while still containing exactly the right people.
	want := []uuid.UUID{a, b}
	sort.Slice(want, func(i, j int) bool { return want[i].String() < want[j].String() })

	online, err := p.Online(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, online)
}

func TestFakePresenceForgetsWhoWentOffline(t *testing.T) {
	ctx := context.Background()
	a := uuid.New()
	p := NewFakePresence(a)

	p.SetOnline()

	online, err := p.Online(ctx)
	require.NoError(t, err)
	assert.Empty(t, online)
}
