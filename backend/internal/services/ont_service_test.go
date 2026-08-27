package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

func ptrFloat64(f float64) *float64 {
	return &f
}

func TestONTService_NewONTService(t *testing.T) {
	db := setupTestDB(t)
	service := NewONTService(db)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.GetDB())
}

func TestONTService_Create(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	err = db.Create(olt).Error
	require.NoError(t, err)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}

	err = ontService.Create(ont)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, ont.ID)
}

func TestONTService_Create_DuplicateSerialNumber(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont1 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "ONT 1",
		Status:       models.ONTStatusUnknown,
	}
	err := ontService.Create(ont1)
	require.NoError(t, err)

	ont2 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       2,
		ONTID:        2,
		SerialNumber: "SN123456",
		Name:         "ONT 2",
		Status:       models.ONTStatusUnknown,
	}
	err = ontService.Create(ont2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestONTService_Create_DuplicatePosition(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont1 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN111111",
		Name:         "ONT 1",
		Status:       models.ONTStatusUnknown,
	}
	err := ontService.Create(ont1)
	require.NoError(t, err)

	ont2 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN222222",
		Name:         "ONT 2",
		Status:       models.ONTStatusUnknown,
	}
	err = ontService.Create(ont2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "position already exists")
}

func TestONTService_Create_InvalidOLT(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)

	ont := &models.ONT{
		OLTID:        uuid.New(),
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}

	err := ontService.Create(ont)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OLT not found")
}

func TestONTService_Create_DefaultStatus(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       "",
	}

	err := ontService.Create(ont)
	require.NoError(t, err)
	assert.Equal(t, models.ONTStatusUnknown, ont.Status)
}

func TestONTService_GetByID(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	found, err := ontService.GetByID(ont.ID)
	require.NoError(t, err)
	assert.Equal(t, ont.ID, found.ID)
	assert.Equal(t, "SN123456", found.SerialNumber)
}

func TestONTService_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)

	_, err := ontService.GetByID(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ONT not found")
}

func TestONTService_GetBySerialNumber(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	found, err := ontService.GetBySerialNumber("SN123456")
	require.NoError(t, err)
	assert.Equal(t, ont.ID, found.ID)
}

func TestONTService_GetBySerialNumber_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)

	_, err := ontService.GetBySerialNumber("NONEXISTENT")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ONT not found")
}

func TestONTService_Update(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	updates := map[string]interface{}{
		"name": "Updated ONT",
	}
	updated, err := ontService.Update(ont.ID, updates)
	require.NoError(t, err)
	assert.Equal(t, "Updated ONT", updated.Name)
}

func TestONTService_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)

	updates := map[string]interface{}{
		"name": "Updated ONT",
	}
	_, err := ontService.Update(uuid.New(), updates)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ONT not found")
}

func TestONTService_Update_SerialNumber_UniqueConstraint(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont1 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN111111",
		Name:         "ONT 1",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont1))

	ont2 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       2,
		ONTID:        2,
		SerialNumber: "SN222222",
		Name:         "ONT 2",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont2))

	updates := map[string]interface{}{
		"serial_number": "SN111111",
	}
	_, err := ontService.Update(ont2.ID, updates)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestONTService_Update_SerialNumber_SameValue(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	updates := map[string]interface{}{
		"serial_number": "SN123456",
	}
	updated, err := ontService.Update(ont.ID, updates)
	require.NoError(t, err)
	assert.Equal(t, "SN123456", updated.SerialNumber)
}

func TestONTService_Delete(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	require.NoError(t, db.Exec(`
		CREATE TABLE ont_metrics (
			time DATETIME NOT NULL,
			ont_id TEXT NOT NULL,
			rx_power REAL
		)
	`).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO ont_metrics (time, ont_id, rx_power) VALUES (?, ?, ?)",
		time.Now(), ont.ID.String(), -20.5,
	).Error)

	err := ontService.Delete(ont.ID)
	require.NoError(t, err)

	_, err = ontService.GetByID(ont.ID)
	assert.Error(t, err)

	var metricsCount int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM ont_metrics WHERE ont_id = ?", ont.ID.String()).Scan(&metricsCount).Error)
	assert.Equal(t, int64(0), metricsCount)
}

func TestONTService_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)

	err := ontService.Delete(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ONT not found")
}

func TestONTService_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)

	onts, total, err := ontService.List(nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, onts, 0)
	assert.Equal(t, int64(0), total)
}

func TestONTService_List_WithData(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	for i := 1; i <= 3; i++ {
		ont := &models.ONT{
			OLTID:        olt.ID,
			PortID:       i,
			ONTID:        i,
			SerialNumber: "SN" + string(rune(48+i)),
			Name:         "ONT " + string(rune(48+i)),
			Status:       models.ONTStatusUnknown,
		}
		require.NoError(t, ontService.Create(ont))
	}

	onts, total, err := ontService.List(nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, onts, 3)
	assert.Equal(t, int64(3), total)
}

func TestONTService_List_WithPagination(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	for i := 1; i <= 5; i++ {
		ont := &models.ONT{
			OLTID:        olt.ID,
			PortID:       i,
			ONTID:        i,
			SerialNumber: "SN" + string(rune(47+i)),
			Name:         "ONT " + string(rune(47+i)),
			Status:       models.ONTStatusUnknown,
		}
		require.NoError(t, ontService.Create(ont))
	}

	onts, total, err := ontService.List(nil, nil, 2, 0)
	require.NoError(t, err)
	assert.Len(t, onts, 2)
	assert.Equal(t, int64(5), total)

	onts, total, err = ontService.List(nil, nil, 2, 2)
	require.NoError(t, err)
	assert.Len(t, onts, 2)
	assert.Equal(t, int64(5), total)
}

func TestONTService_List_FilterByOLT(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt1 := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "OLT 1",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt1)

	olt2 := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "OLT 2",
		IPAddress:         "192.168.1.2",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt2)

	ont1 := &models.ONT{
		OLTID:        olt1.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN111111",
		Name:         "ONT 1",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont1))

	ont2 := &models.ONT{
		OLTID:        olt2.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN222222",
		Name:         "ONT 2",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont2))

	onts, total, err := ontService.List(&olt1.ID, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, onts, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, olt1.ID, onts[0].OLTID)
}

func TestONTService_List_FilterByStatus(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont1 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN111111",
		Name:         "ONT 1",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, ontService.Create(ont1))

	ont2 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       2,
		ONTID:        2,
		SerialNumber: "SN222222",
		Name:         "ONT 2",
		Status:       models.ONTStatusOffline,
	}
	require.NoError(t, ontService.Create(ont2))

	status := models.ONTStatusOnline
	onts, total, err := ontService.List(nil, &status, 10, 0)
	require.NoError(t, err)
	assert.Len(t, onts, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, models.ONTStatusOnline, onts[0].Status)
}

func TestONTService_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	err := ontService.UpdateStatus(ont.ID, models.ONTStatusOnline)
	require.NoError(t, err)

	updated, _ := ontService.GetByID(ont.ID)
	assert.Equal(t, models.ONTStatusOnline, updated.Status)
	assert.NotNil(t, updated.LastSeenAt)
}

func TestONTService_UpdateStatus_Offline(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	lastSeenTime := time.Now().Add(-1 * time.Hour)
	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusOnline,
		LastSeenAt:   &lastSeenTime,
	}
	require.NoError(t, ontService.Create(ont))

	err := ontService.UpdateStatus(ont.ID, models.ONTStatusOffline)
	require.NoError(t, err)

	updated, _ := ontService.GetByID(ont.ID)
	assert.Equal(t, models.ONTStatusOffline, updated.Status)
	assert.Equal(t, lastSeenTime.Unix(), updated.LastSeenAt.Unix())
}

func TestONTService_GetByOLTAndPosition(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        5,
		SerialNumber: "SN123456",
		Name:         "Test ONT",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	found, err := ontService.GetByOLTAndPosition(olt.ID, 1, 5)
	require.NoError(t, err)
	assert.Equal(t, ont.ID, found.ID)
}

func TestONTService_GetByOLTAndPosition_NotFound(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	_, err := ontService.GetByOLTAndPosition(olt.ID, 99, 99)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ONT not found at position")
}

func TestONTService_BulkRegisterFromDiscovery_New(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	discovered := []connectivity.DiscoveredONT{
		{
			PortID:          1,
			ONTID:           1,
			SerialNumber:    "SN111111",
			Name:            "New ONT 1",
			Description:     "Test ONT",
			DeviceType:      "HG6421",
			HardwareVersion: "1.0",
			SoftwareVersion: "2.0",
			IPAddress:       "192.168.1.100",
			MACAddress:      "00:11:22:33:44:55",
		},
		{
			PortID:       2,
			ONTID:        1,
			SerialNumber: "SN222222",
			Name:         "New ONT 2",
			Description:  "Test ONT 2",
		},
	}

	result := ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
	assert.Equal(t, 2, result.Registered)
	assert.Equal(t, 0, result.Skipped)
	assert.Len(t, result.Errors, 0)
}

func TestONTService_BulkRegisterFromDiscovery_Update(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	existing := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN111111",
		Name:         "Old Name",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(existing))

	discovered := []connectivity.DiscoveredONT{
		{
			PortID:       1,
			ONTID:        1,
			SerialNumber: "SN111111",
			Name:         "Updated Name",
			Description:  "Updated Description",
		},
	}

	result := ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
	assert.Equal(t, 1, result.Registered)
	assert.Equal(t, 0, result.Skipped)

	updated, _ := ontService.GetByID(existing.ID)
	assert.Equal(t, "Updated Name", updated.Name)
}

func TestONTService_BulkRegisterFromDiscovery_Skip(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	existing := &models.ONT{
		OLTID:           olt.ID,
		PortID:          1,
		ONTID:           1,
		SerialNumber:    "SN111111",
		Name:            "Name",
		Description:     "Description",
		DeviceType:      "HG6421",
		HardwareVersion: "1.0",
		SoftwareVersion: "2.0",
		IPAddress:       "192.168.1.100",
		MACAddress:      "00:11:22:33:44:55",
		Status:          models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(existing))

	discovered := []connectivity.DiscoveredONT{
		{
			PortID:          1,
			ONTID:           1,
			SerialNumber:    "SN111111",
			Name:            "Name",
			Description:     "Description",
			DeviceType:      "HG6421",
			HardwareVersion: "1.0",
			SoftwareVersion: "2.0",
			IPAddress:       "192.168.1.100",
			MACAddress:      "00:11:22:33:44:55",
		},
	}

	result := ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
	assert.Equal(t, 0, result.Registered)
	assert.Equal(t, 1, result.Skipped)
}

func TestONTService_BulkRegisterFromDiscovery_DuplicateSerial(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	existing := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN111111",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(existing))

	discovered := []connectivity.DiscoveredONT{
		{
			PortID:       2,
			ONTID:        1,
			SerialNumber: "SN111111",
		},
	}

	result := ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
	assert.Equal(t, 0, result.Registered)
	assert.Equal(t, 0, result.Skipped)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "already exists")
}

func TestONTService_GetByID_DatabaseError(t *testing.T) {
	db := setupTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	brokenService := NewONTService(db.WithContext(ctx))
	_, err := brokenService.GetByID(uuid.New())
	assert.Error(t, err)
}

func TestONTService_Create_AllFields(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:           olt.ID,
		PortID:          1,
		ONTID:           1,
		SerialNumber:    "SN123456",
		Name:            "Test ONT",
		Description:     "Test Description",
		DeviceType:      "HG6421",
		HardwareVersion: "1.0",
		SoftwareVersion: "2.0",
		IPAddress:       "192.168.1.100",
		MACAddress:      "00:11:22:33:44:55",
		Status:          models.ONTStatusUnknown,
		RxPower:         ptrFloat64(-20.5),
		TxPower:         ptrFloat64(3.2),
		Distance:        1200,
	}

	err := ontService.Create(ont)
	require.NoError(t, err)

	found, _ := ontService.GetByID(ont.ID)
	assert.Equal(t, "Test ONT", found.Name)
	assert.Equal(t, "Test Description", found.Description)
	assert.Equal(t, "HG6421", found.DeviceType)
	assert.Equal(t, "192.168.1.100", found.IPAddress)
}

func TestONTService_Update_MultipleFields(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Name:         "Original",
		Description:  "Original Description",
		Status:       models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(ont))

	updates := map[string]interface{}{
		"name":        "Updated",
		"description": "Updated Description",
		"ip_address":  "192.168.1.200",
	}
	updated, err := ontService.Update(ont.ID, updates)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "Updated Description", updated.Description)
	assert.Equal(t, "192.168.1.200", updated.IPAddress)
}

func TestONTService_BulkRegisterFromDiscovery_PartialUpdate(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	existing := &models.ONT{
		OLTID:           olt.ID,
		PortID:          1,
		ONTID:           1,
		SerialNumber:    "SN111111",
		Name:            "Original",
		HardwareVersion: "1.0",
		Status:          models.ONTStatusUnknown,
	}
	require.NoError(t, ontService.Create(existing))

	discovered := []connectivity.DiscoveredONT{
		{
			PortID:          1,
			ONTID:           1,
			SerialNumber:    "SN111111",
			Name:            "Updated",
			HardwareVersion: "1.0",
			SoftwareVersion: "2.0",
		},
	}

	result := ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
	assert.Equal(t, 1, result.Registered)

	updated, _ := ontService.GetByID(existing.ID)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "2.0", updated.SoftwareVersion)
}

func TestONTService_UpdateUptimeMetrics_Online(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	lastOnline := time.Now().Add(-3600 * time.Second)
	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Status:       models.ONTStatusOnline,
		LastOnline:   &lastOnline,
	}
	require.NoError(t, ontService.Create(ont))

	err := ontService.UpdateUptimeMetrics(ont.ID)
	assert.NoError(t, err)

	updated, _ := ontService.GetByID(ont.ID)
	assert.Greater(t, updated.Uptime, int64(3500))
}

func TestONTService_UpdateUptimeMetrics_Offline(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	lastOffline := time.Now().Add(-1800 * time.Second)
	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Status:       models.ONTStatusOffline,
		LastOffline:  &lastOffline,
	}
	require.NoError(t, ontService.Create(ont))

	err := ontService.UpdateUptimeMetrics(ont.ID)
	assert.NoError(t, err)

	updated, _ := ontService.GetByID(ont.ID)
	assert.Greater(t, updated.LastDownTimeDuration, int64(1700))
}

func TestONTService_UpdateUptimeMetrics_NoTimestamps(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN123456",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, ontService.Create(ont))

	err := ontService.UpdateUptimeMetrics(ont.ID)
	assert.NoError(t, err)
}

func TestONTService_ListONTSummariesForOLT(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	ontService := NewONTService(db)

	site, _ := siteService.Create("Test Site", "Test Location", "Test Description")

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            site.ID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	db.Create(olt)

	ont1 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN111111",
		Name:         "ONT 1",
		Description:  "First ONT",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, ontService.Create(ont1))

	ont2 := &models.ONT{
		OLTID:        olt.ID,
		PortID:       2,
		ONTID:        2,
		SerialNumber: "SN222222",
		Name:         "ONT 2",
		Description:  "Second ONT",
		Status:       models.ONTStatusOffline,
	}
	require.NoError(t, ontService.Create(ont2))

	onts, err := ontService.ListONTSummariesForOLT(olt.ID)
	require.NoError(t, err)
	assert.Len(t, onts, 2)

	// Verify the projection fields
	assert.Equal(t, 1, onts[0].PortID)
	assert.Equal(t, 1, onts[0].ONTID)
	assert.Equal(t, "SN111111", onts[0].SerialNumber)
	assert.Equal(t, "online", onts[0].Status)
	assert.Equal(t, "ONT 1", onts[0].Name)
	assert.Equal(t, "First ONT", onts[0].Description)

	assert.Equal(t, 2, onts[1].PortID)
	assert.Equal(t, 2, onts[1].ONTID)
	assert.Equal(t, "SN222222", onts[1].SerialNumber)
	assert.Equal(t, "offline", onts[1].Status)
	assert.Equal(t, "ONT 2", onts[1].Name)
	assert.Equal(t, "Second ONT", onts[1].Description)
}

func TestONTService_PruneMissingFromDiscoveryDeletesStaleONTs(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	siteService := NewSiteService(db)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	oltID := uuid.New()
	otherOLTID := uuid.New()
	require.NoError(t, db.Create(&models.OLT{ID: oltID, SiteID: site.ID, Name: "OLT 1", IPAddress: "192.168.1.1", SNMPCommunity: "public", PreferredProtocol: models.OLTProtocolSSH}).Error)
	require.NoError(t, db.Create(&models.OLT{ID: otherOLTID, SiteID: site.ID, Name: "OLT 2", IPAddress: "192.168.1.2", SNMPCommunity: "public", PreferredProtocol: models.OLTProtocolSSH}).Error)

	kept := &models.ONT{OLTID: oltID, PortID: 1, ONTID: 1, SerialNumber: "KEEP001", Status: models.ONTStatusOnline}
	stale := &models.ONT{OLTID: oltID, PortID: 1, ONTID: 2, SerialNumber: "STALE001", Status: models.ONTStatusOnline}
	otherOLTONT := &models.ONT{OLTID: otherOLTID, PortID: 1, ONTID: 2, SerialNumber: "OTHER001", Status: models.ONTStatusOnline}
	require.NoError(t, ontService.Create(kept))
	require.NoError(t, ontService.Create(stale))
	require.NoError(t, ontService.Create(otherOLTONT))
	require.NoError(t, db.Exec(`
		CREATE TABLE ont_metrics (
			time DATETIME NOT NULL,
			ont_id TEXT NOT NULL,
			rx_power REAL
		)
	`).Error)
	require.NoError(t, db.Exec("INSERT INTO ont_metrics (time, ont_id, rx_power) VALUES (?, ?, ?)", time.Now(), stale.ID.String(), -20.5).Error)
	require.NoError(t, db.Create(&models.ONTEvent{ONTID: stale.ID, EventType: models.EventTypeOffline, EventTime: time.Now(), Reason: "stale"}).Error)

	deleted, err := ontService.PruneMissingFromDiscovery(oltID, []connectivity.DiscoveredONT{
		{PortID: 1, ONTID: 1, SerialNumber: "KEEP001"},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", kept.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", stale.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", otherOLTONT.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM ont_metrics WHERE ont_id = ?", stale.ID.String()).Scan(&count).Error)
	assert.Equal(t, int64(0), count)
	require.NoError(t, db.Model(&models.ONTEvent{}).Where("ont_id = ?", stale.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

// A contended SNMP walk on a busy C300 returns the phase-state table empty
// without failing. Treating that as "the OLT has no ONTs" once deleted a whole
// 198-ONT inventory along with its event history.
func TestONTService_PruneMissingFromDiscoveryKeepsONTsWhenDiscoveryIsEmpty(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	siteService := NewSiteService(db)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	oltID := uuid.New()
	require.NoError(t, db.Create(&models.OLT{ID: oltID, SiteID: site.ID, Name: "OLT 1", IPAddress: "192.168.1.1", SNMPCommunity: "public", PreferredProtocol: models.OLTProtocolSSH}).Error)

	ont := &models.ONT{OLTID: oltID, PortID: 1, ONTID: 1, SerialNumber: "KEEP001", Status: models.ONTStatusOnline}
	require.NoError(t, ontService.Create(ont))

	deleted, err := ontService.PruneMissingFromDiscovery(oltID, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", ont.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "an empty discovery must not delete the inventory")
}
