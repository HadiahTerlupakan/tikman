package services

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
)

var flowMinute = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

func TestWriteMinuteStoresTheOtherPOPAsNull(t *testing.T) {
	db := setupTestDB(t)
	routerID, pop, upstream := uuid.New(), uuid.New(), uuid.New()
	rows := []netflow.MinuteRow{
		{Time: flowMinute, MinuteKey: netflow.MinuteKey{RouterID: routerID, POP: pop, Upstream: upstream,
			Direction: netflow.Download, ASN: 64496}, Bytes: 1500, Packets: 1},
		{Time: flowMinute, MinuteKey: netflow.MinuteKey{RouterID: routerID, Upstream: upstream,
			Direction: netflow.Upload, ASN: netflow.OtherASN}, Bytes: 80, Packets: 1},
	}

	require.NoError(t, NewFlowStore(db).WriteMinute(context.Background(), rows))

	var stored []models.RouterFlowMinute
	require.NoError(t, db.Order("bytes DESC").Find(&stored).Error)
	require.Len(t, stored, 2)
	require.NotNil(t, stored[0].POPInterfaceID)
	assert.Equal(t, pop, *stored[0].POPInterfaceID)
	assert.Equal(t, "download", stored[0].Direction)
	assert.EqualValues(t, 64496, stored[0].ASN)
	assert.Nil(t, stored[1].POPInterfaceID, "uuid.Nil is the Lainnya POP")
	assert.Equal(t, "upload", stored[1].Direction)
}

func TestWriteTopIPsStoresTheAddress(t *testing.T) {
	db := setupTestDB(t)
	routerID := uuid.New()
	rows := []netflow.TopIPRow{{Time: flowMinute, RouterID: routerID, Direction: netflow.Download,
		IP: netip.MustParseAddr("203.0.113.5"), ASN: 64498, Bytes: 9000}}

	require.NoError(t, NewFlowStore(db).WriteTopIPs(context.Background(), rows))

	var stored models.RouterFlowTopIP
	require.NoError(t, db.First(&stored, "router_id = ?", routerID).Error)
	assert.Equal(t, "203.0.113.5", stored.IP)
	assert.Nil(t, stored.POPInterfaceID)
	assert.EqualValues(t, 9000, stored.Bytes)
}

func TestMarkSeenTouchesOnlyTheGivenRouters(t *testing.T) {
	routers, db := newRouterServiceForTest(t)
	heard, err := routers.Create(validRouterInput())
	require.NoError(t, err)
	other := validRouterInput()
	other.Name, other.Host = "border-2", "192.0.2.2"
	silent, err := routers.Create(other)
	require.NoError(t, err)

	require.NoError(t, NewFlowStore(db).MarkSeen(context.Background(), []uuid.UUID{heard.ID}, flowMinute))

	stored, err := routers.Get(heard.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.FlowLastSeenAt)
	assert.True(t, flowMinute.Equal(*stored.FlowLastSeenAt))
	quiet, err := routers.Get(silent.ID)
	require.NoError(t, err)
	assert.Nil(t, quiet.FlowLastSeenAt)
}

func TestSaveASNNamesReplacesNamesItAlreadyHas(t *testing.T) {
	db := setupTestDB(t)
	store := NewFlowStore(db)
	ctx := context.Background()
	require.NoError(t, store.SaveASNNames(ctx, map[uint32]netflow.ASNInfo{64496: {Name: "OLD-NAME", Country: "ZZ"}}, flowMinute))

	later := flowMinute.Add(24 * time.Hour)
	require.NoError(t, store.SaveASNNames(ctx, map[uint32]netflow.ASNInfo{
		64496: {Name: "EXAMPLE-NET-A", Country: "ZZ"}, 64497: {Name: "EXAMPLE-NET-B", Country: "ZZ"},
	}, later))

	var names []models.ASNName
	require.NoError(t, db.Order("asn").Find(&names).Error)
	require.Len(t, names, 2)
	assert.Equal(t, "EXAMPLE-NET-A", names[0].Name)
	assert.True(t, later.Equal(names[0].UpdatedAt))
}
