package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func validZTEGPONRequest() models.ZTEGPONRegisterRequest {
	return models.ZTEGPONRegisterRequest{
		OLTID:          uuid.New(),
		Card:           1,
		PON:            1,
		ONUIDMode:      models.ZTEONUIDAuto,
		SerialNumber:   " zteg12345678 ",
		ONUType:        "F601",
		ServiceEnabled: true,
		VLANMode:       "tag",
		ServiceType:    "internet",
		VLANID:         100,
		WANMode:        "pppoe",
		PPPoEUsername:  "subscriber01",
		PPPoEPassword:  "secret-pass",
	}
}

func validZTEOLT(model models.OLTModel) *models.OLT {
	return &models.OLT{Model: model}
}

func TestValidateZTEGPONRegister(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*models.ZTEGPONRegisterRequest, *models.OLT)
		wantSubstr string
	}{
		{name: "accepts C300", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { olt.Model = models.OLTModelZTEC300 }},
		{name: "accepts C320", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { olt.Model = models.OLTModelZTEC320 }},
		{name: "rejects HSGQ", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { olt.Model = models.OLTModelHSGQ }, wantSubstr: "C300 or C320"},
		{name: "requires card", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.Card = 0 }, wantSubstr: "card"},
		{name: "requires PON", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.PON = 0 }, wantSubstr: "PON"},
		{name: "requires serial number", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.SerialNumber = "" }, wantSubstr: "serial number"},
		{name: "rejects custom ONU ID below range", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) {
			req.ONUIDMode = models.ZTEONUIDCustom
			req.ONUID = 0
		}, wantSubstr: "1-127"},
		{name: "rejects custom ONU ID above range", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) {
			req.ONUIDMode = models.ZTEONUIDCustom
			req.ONUID = 128
		}, wantSubstr: "1-127"},
		{name: "auto mode accepts zero ONU ID", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) {
			req.ONUIDMode = models.ZTEONUIDAuto
			req.ONUID = 0
		}},
		{name: "rejects nonzero auto ONU ID", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) {
			req.ONUIDMode = models.ZTEONUIDAuto
			req.ONUID = 1
		}, wantSubstr: "auto ONU ID must be zero"},
		{name: "normalizes lowercase serial after trimming", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.SerialNumber = " zteg12345678 " }},
		{name: "rejects serial with wrong length", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.SerialNumber = "ZTEG1234567" }, wantSubstr: "12 uppercase alphanumeric"},
		{name: "rejects serial with punctuation", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.SerialNumber = "ZTEG1234567!" }, wantSubstr: "12 uppercase alphanumeric"},
		{name: "requires ONU type", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.ONUType = "" }, wantSubstr: "ONU type"},
		{name: "limits ONU type length", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.ONUType = strings.Repeat("a", 65) }, wantSubstr: "64"},
		{name: "requires service enabled", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.ServiceEnabled = false }, wantSubstr: "service must be enabled"},
		{name: "requires VLAN in range", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.VLANID = 4095 }, wantSubstr: "VLAN ID"},
		{name: "rejects unsupported WAN mode", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.WANMode = "dhcp" }, wantSubstr: "pppoe"},
		{name: "requires internet service", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.ServiceType = "voip" }, wantSubstr: "internet"},
		{name: "requires PPPoE username", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.PPPoEUsername = "" }, wantSubstr: "PPPoE username"},
		{name: "rejects whitespace in PPPoE username", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.PPPoEUsername = "subscriber 01" }, wantSubstr: "PPPoE username"},
		{name: "requires PPPoE password without exposing it", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.PPPoEPassword = "" }, wantSubstr: "PPPoE password"},
		{name: "rejects whitespace in PPPoE password without exposing it", mutate: func(req *models.ZTEGPONRegisterRequest, olt *models.OLT) { req.PPPoEPassword = "secret pass" }, wantSubstr: "PPPoE password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validZTEGPONRequest()
			olt := validZTEOLT(models.OLTModelZTEC300)
			tt.mutate(&req, olt)

			err := ValidateZTEGPONRegister(req, olt)
			if tt.wantSubstr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantSubstr)
			if req.PPPoEPassword != "" {
				assert.NotContains(t, err.Error(), req.PPPoEPassword)
			}
		})
	}
}

func setupZTEProvisionDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ONT{}))
	return db
}

func TestResolveZTEONUID(t *testing.T) {
	db := setupZTEProvisionDB(t)
	oltID := uuid.New()
	require.NoError(t, db.Create(&models.ONT{OLTID: oltID, PortID: 1, ONTID: 1, SerialNumber: "ZTEG00000001"}).Error)
	require.NoError(t, db.Create(&models.ONT{OLTID: oltID, PortID: 1, ONTID: 2, SerialNumber: "ZTEG00000002"}).Error)

	t.Run("auto returns first free ONU ID on port", func(t *testing.T) {
		id, err := ResolveZTEONUID(context.Background(), db, oltID, 1, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, id)
	})
	t.Run("auto ignores IDs used on another port", func(t *testing.T) {
		require.NoError(t, db.Create(&models.ONT{OLTID: oltID, PortID: 2, ONTID: 1, SerialNumber: "ZTEG00000003"}).Error)
		id, err := ResolveZTEONUID(context.Background(), db, oltID, 2, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, id)
	})
	t.Run("custom rejects used ONU ID", func(t *testing.T) {
		_, err := ResolveZTEONUID(context.Background(), db, oltID, 1, 2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already used")
	})
	t.Run("custom accepts free ONU ID", func(t *testing.T) {
		id, err := ResolveZTEONUID(context.Background(), db, oltID, 1, 127)
		require.NoError(t, err)
		assert.Equal(t, 127, id)
	})
}

func TestResolveZTEONUIDRejectsInvalidRequestedID(t *testing.T) {
	db := setupZTEProvisionDB(t)
	oltID := uuid.New()
	for _, requestedID := range []int{-1, 128} {
		_, err := ResolveZTEONUID(context.Background(), db, oltID, 1, requestedID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1-127")
	}
}
