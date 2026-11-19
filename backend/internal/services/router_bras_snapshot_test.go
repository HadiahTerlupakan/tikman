package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
)

func TestPoolTotalFromRanges(t *testing.T) {
	cases := []struct {
		ranges string
		want   *int
	}{
		{"10.0.0.1-10.0.0.254", intPtr(254)},
		{"10.0.0.1-10.0.0.10,10.0.1.1", intPtr(11)},
		{"10.0.0.1-10.0.0.10, 10.0.1.1", intPtr(11)},
		{"10.0.0.10-10.0.0.1", nil},
		{"2001:db8::1-2001:db8::ff", nil},
		{"", nil},
		{"garbage", nil},
	}
	for _, c := range cases {
		t.Run(c.ranges, func(t *testing.T) {
			assert.Equal(t, c.want, poolTotalFromRanges(c.ranges))
		})
	}
}

// A range that counts fewer addresses than are in use is not the pool's size,
// and a negative free count would read as an exhausted pool.
func TestReadPoolRangesShorterThanUsedIsUnknown(t *testing.T) {
	api := &fakeRouterAPI{poolUsed: map[string]int{"pel": 20}}

	pool, err := readPool(api, mikrotik.IPPool{Name: "pel", Ranges: "192.0.2.1-192.0.2.10"})

	require.NoError(t, err)
	assert.Equal(t, 20, pool.Used)
	assert.Nil(t, pool.Total)
	assert.Nil(t, pool.Free)
}

func TestRadiusAnsweringVerdict(t *testing.T) {
	yes, no := true, false
	prev := BRASRadius{Requests: 100, Accepts: 80, Rejects: 10, Timeouts: 10}
	cases := []struct {
		name string
		prev *BRASRadius
		cur  BRASRadius
		want *bool
	}{
		{"no previous snapshot", nil, BRASRadius{Requests: 120}, nil},
		{"requests grew, only timeouts followed", &prev,
			BRASRadius{Requests: 130, Accepts: 80, Rejects: 10, Timeouts: 40}, &no},
		{"requests grew and accepts followed", &prev,
			BRASRadius{Requests: 130, Accepts: 100, Rejects: 10, Timeouts: 40}, &yes},
		{"requests grew and rejects followed", &prev,
			BRASRadius{Requests: 130, Accepts: 80, Rejects: 20, Timeouts: 10}, &yes},
		{"no new requests keeps the earlier verdict", &BRASRadius{Requests: 100, Answering: &no},
			BRASRadius{Requests: 100}, &no},
		{"no new requests and no earlier verdict", &prev, BRASRadius{Requests: 100}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, radiusAnswering(c.prev, c.cur))
		})
	}
}
