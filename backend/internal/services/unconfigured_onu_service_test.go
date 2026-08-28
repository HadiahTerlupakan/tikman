package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func seedUncfgOLT(t *testing.T, db *gorm.DB, community string) *models.OLT {
	t.Helper()

	site, err := NewSiteService(db).Create("Uncfg Site", "Location", "Description")
	require.NoError(t, err)

	olt := &models.OLT{
		ID:            uuid.New(),
		SiteID:        site.ID,
		Name:          "Uncfg OLT",
		IPAddress:     "192.168.1.10",
		SNMPCommunity: community,
		SNMPPort:      161,
		Username:      "admin",
		Password:      "encrypted",
		SSHPort:       22,
		TelnetPort:    23,
	}
	require.NoError(t, db.Create(olt).Error)

	// The snmp_community column defaults to "public", so an unset community has
	// to be written back explicitly to model an OLT that lacks one.
	if community == "" {
		require.NoError(t, db.Model(olt).Update("snmp_community", "").Error)
	}

	return olt
}

func TestUnconfiguredONUService_ListByOLT(t *testing.T) {
	db := setupTestDB(t)
	olt := seedUncfgOLT(t, db, "public")

	var gotIP, gotCommunity string
	var gotPort int
	service := &UnconfiguredONUService{
		db: db,
		walk: func(_ connectivity.Driver, ipAddress, community string, snmpPort int) ([]connectivity.UnconfiguredONU, error) {
			gotIP, gotCommunity, gotPort = ipAddress, community, snmpPort
			return []connectivity.UnconfiguredONU{
				{Slot: 3, Port: 1, SerialNumber: "HWTCB403E8A0", DeviceType: "HG8245H5"},
			}, nil
		},
	}

	onus, err := service.ListByOLT(olt.ID)
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.10", gotIP)
	assert.Equal(t, "public", gotCommunity)
	assert.Equal(t, 161, gotPort)

	require.Len(t, onus, 1)
	assert.Equal(t, "HWTCB403E8A0", onus[0].SerialNumber)
	assert.Equal(t, "HG8245H5", onus[0].DeviceType)
	assert.Equal(t, 3, onus[0].Slot)
}

// An OLT lists an ONU as autofind only when it holds no configuration for it,
// so that listing outranks TikMan's own records. Filtering it against them hid
// the case this exists for: an ONU deleted from the OLT while a stale row
// survived here stayed invisible and could never be provisioned again.
func TestUnconfiguredONUService_TrustsTheOLTOverAStaleLocalRow(t *testing.T) {
	db := setupTestDB(t)
	olt := seedUncfgOLT(t, db, "public")

	require.NoError(t, NewONTService(db).Create(&models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        7,
		SerialNumber: "HWTCB403E8A0",
		Status:       models.ONTStatusOnline,
	}))

	service := &UnconfiguredONUService{
		db: db,
		walk: func(connectivity.Driver, string, string, int) ([]connectivity.UnconfiguredONU, error) {
			return []connectivity.UnconfiguredONU{
				{Slot: 3, Port: 1, SerialNumber: "HWTCB403E8A0"},
				{Slot: 3, Port: 1, SerialNumber: "ZTEGCAFFC2FD"},
			}, nil
		},
	}

	onus, err := service.ListByOLT(olt.ID)

	require.NoError(t, err)
	require.Len(t, onus, 2, "an ONU the OLT calls unconfigured must be listed")
	assert.Equal(t, "HWTCB403E8A0", onus[0].SerialNumber)
}

func TestUnconfiguredONUService_KeepsSerialRegisteredOnAnotherOLT(t *testing.T) {
	db := setupTestDB(t)
	olt := seedUncfgOLT(t, db, "public")
	otherOLT := seedUncfgOLT(t, db, "public")

	require.NoError(t, NewONTService(db).Create(&models.ONT{
		OLTID:        otherOLT.ID,
		PortID:       1,
		ONTID:        7,
		SerialNumber: "HWTCB403E8A0",
		Status:       models.ONTStatusOnline,
	}))

	service := &UnconfiguredONUService{
		db: db,
		walk: func(connectivity.Driver, string, string, int) ([]connectivity.UnconfiguredONU, error) {
			return []connectivity.UnconfiguredONU{
				{Slot: 3, Port: 1, SerialNumber: "HWTCB403E8A0"},
			}, nil
		},
	}

	onus, err := service.ListByOLT(olt.ID)
	require.NoError(t, err)
	require.Len(t, onus, 1)
}

func TestUnconfiguredONUService_EmptyResultIsNotNil(t *testing.T) {
	db := setupTestDB(t)
	olt := seedUncfgOLT(t, db, "public")

	service := &UnconfiguredONUService{
		db: db,
		walk: func(connectivity.Driver, string, string, int) ([]connectivity.UnconfiguredONU, error) {
			return nil, nil
		},
	}

	onus, err := service.ListByOLT(olt.ID)
	require.NoError(t, err)
	assert.NotNil(t, onus)
	assert.Empty(t, onus)
}

func TestUnconfiguredONUService_MissingOLT(t *testing.T) {
	db := setupTestDB(t)

	service := &UnconfiguredONUService{
		db: db,
		walk: func(connectivity.Driver, string, string, int) ([]connectivity.UnconfiguredONU, error) {
			t.Fatal("walk must not run when the OLT is unknown")
			return nil, nil
		},
	}

	_, err := service.ListByOLT(uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OLT not found")
}

func TestUnconfiguredONUService_MissingCommunity(t *testing.T) {
	db := setupTestDB(t)
	olt := seedUncfgOLT(t, db, "")

	service := &UnconfiguredONUService{
		db: db,
		walk: func(connectivity.Driver, string, string, int) ([]connectivity.UnconfiguredONU, error) {
			t.Fatal("walk must not run without an SNMP community")
			return nil, nil
		},
	}

	_, err := service.ListByOLT(olt.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SNMP community not configured")
}

func TestUnconfiguredONUService_WalkFailurePropagates(t *testing.T) {
	db := setupTestDB(t)
	olt := seedUncfgOLT(t, db, "public")

	service := &UnconfiguredONUService{
		db: db,
		walk: func(connectivity.Driver, string, string, int) ([]connectivity.UnconfiguredONU, error) {
			return nil, errors.New("SNMP connect failed")
		},
	}

	_, err := service.ListByOLT(olt.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SNMP connect failed")
}

func TestNewUnconfiguredONUService_UsesLiveWalker(t *testing.T) {
	service := NewUnconfiguredONUService(setupTestDB(t))
	assert.NotNil(t, service.walk)
}
