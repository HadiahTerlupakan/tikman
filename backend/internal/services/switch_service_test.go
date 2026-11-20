package services

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func newSwitchServiceForTest(t *testing.T) (*SwitchService, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	return NewSwitchService(db, routerTestKey), db
}

// testSSHFingerprint has the shape ssh-keygen -l prints: SHA256: and 43
// base64 characters.
const testSSHFingerprint = "SHA256:bmV3LWtleS1mb3ItdGVzdHMtb25seS0xMjM0NTY3ODk"

func validSwitchInput() SwitchInput {
	return SwitchInput{
		Name: "SW-TEST", Host: "192.0.2.10", Vendor: models.SwitchVendorHuaweiCE,
		SNMPCommunity: "community-secret", Protocol: models.SwitchProtocolTelnet,
		Username: "tikman", Password: "password-secret",
	}
}

func TestCreateSwitchEncryptsSecretsAndKeepsThemOutOfJSON(t *testing.T) {
	service, db := newSwitchServiceForTest(t)

	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)

	var stored models.Switch
	require.NoError(t, db.First(&stored, "id = ?", sw.ID).Error)
	assert.NotEqual(t, "community-secret", stored.SNMPCommunity)
	assert.NotEqual(t, "password-secret", stored.Password)
	community, password, err := service.Secrets(&stored)
	require.NoError(t, err)
	assert.Equal(t, "community-secret", community)
	assert.Equal(t, "password-secret", password)

	raw, err := json.Marshal(sw)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "secret")
	assert.NotContains(t, string(raw), stored.SNMPCommunity)
	assert.NotContains(t, string(raw), stored.Password)
}

func TestCreateSwitchAppliesDefaultsAndStartsUnknown(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)

	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)

	assert.Equal(t, 161, sw.SNMPPort)
	assert.Equal(t, 23, sw.Port)
	assert.True(t, sw.Enabled)
	assert.False(t, sw.WritesEnabled)
	assert.Equal(t, models.SwitchStatusUnknown, sw.Status)
}

func TestCreateSwitchAcceptsTelnetAndSSHForHuaweiCE(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	for _, protocol := range []models.SwitchProtocol{models.SwitchProtocolTelnet, models.SwitchProtocolSSH} {
		in := validSwitchInput()
		in.Protocol = protocol

		sw, err := service.Create(in)

		require.NoError(t, err, protocol)
		assert.Equal(t, protocol, sw.Protocol)
	}
}

func TestCreateSwitchRefusesWhatIsNotSupportedYet(t *testing.T) {
	cases := map[string]func(*SwitchInput){
		"huawei_ce over rest": func(in *SwitchInput) { in.Protocol = models.SwitchProtocolREST },
		"crs":                 func(in *SwitchInput) { in.Vendor = models.SwitchVendorCRS },
		"fiberhome":           func(in *SwitchInput) { in.Vendor = models.SwitchVendorFiberHome },
		"unknown vendor":      func(in *SwitchInput) { in.Vendor = "cisco" },
		"no community":        func(in *SwitchInput) { in.SNMPCommunity = "" },
		"no password":         func(in *SwitchInput) { in.Password = "" },
		"blank name":          func(in *SwitchInput) { in.Name = "  " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			service, db := newSwitchServiceForTest(t)
			in := validSwitchInput()
			mutate(&in)

			_, err := service.Create(in)

			assert.ErrorIs(t, err, ErrValidation)
			var count int64
			db.Model(&models.Switch{}).Count(&count)
			assert.Zero(t, count)
		})
	}
}

func TestCreateSwitchSaysWhyCRSAndFiberHomeAreRefused(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	in := validSwitchInput()
	in.Vendor = models.SwitchVendorCRS

	_, err := service.Create(in)

	assert.ErrorContains(t, err, "belum didukung")
}

func TestCreateSwitchSeedsOneJobPerKind(t *testing.T) {
	service, db := newSwitchServiceForTest(t)

	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)

	var kinds []models.SwitchPollKind
	require.NoError(t, db.Model(&models.SwitchPollJob{}).Where("switch_id = ?", sw.ID).Order("kind").Pluck("kind", &kinds).Error)
	assert.Equal(t, []models.SwitchPollKind{
		models.SwitchPollKindBackup, models.SwitchPollKindInterfaces, models.SwitchPollKindInventory}, kinds)
}

func TestUpdateSwitchKeepsASecretLeftEmptyAndReplacesAGivenOne(t *testing.T) {
	service, db := newSwitchServiceForTest(t)
	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)
	empty, fresh, name := "", "new-password", "SW-RENAMED"

	_, err = service.Update(sw.ID, SwitchUpdate{Name: &name, SNMPCommunity: &empty, Password: &fresh})
	require.NoError(t, err)

	var stored models.Switch
	require.NoError(t, db.First(&stored, "id = ?", sw.ID).Error)
	community, password, err := service.Secrets(&stored)
	require.NoError(t, err)
	assert.Equal(t, "SW-RENAMED", stored.Name)
	assert.Equal(t, "community-secret", community)
	assert.Equal(t, "new-password", password)
}

func TestUpdateSwitchRefusesAProtocolTheVendorLacks(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)
	rest := models.SwitchProtocolREST

	_, err = service.Update(sw.ID, SwitchUpdate{Protocol: &rest})

	assert.ErrorIs(t, err, ErrValidation)
}

func TestUpdateSwitchChecksTheRackSlot(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)
	slot := 100

	_, err = service.Update(sw.ID, SwitchUpdate{RackPosition: &slot})

	assert.ErrorIs(t, err, ErrValidation)
}

func TestDeleteSwitchTakesItsJobsAndRowsWithIt(t *testing.T) {
	service, db := newSwitchServiceForTest(t)
	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.SwitchVLAN{SwitchID: sw.ID, VID: 10, Name: "MGMT"}).Error)
	other, err := service.Create(validSwitchInput())
	require.NoError(t, err)

	require.NoError(t, service.Delete(sw.ID))

	var jobs, vlans, otherJobs int64
	db.Model(&models.SwitchPollJob{}).Where("switch_id = ?", sw.ID).Count(&jobs)
	db.Model(&models.SwitchVLAN{}).Where("switch_id = ?", sw.ID).Count(&vlans)
	db.Model(&models.SwitchPollJob{}).Where("switch_id = ?", other.ID).Count(&otherJobs)
	assert.Zero(t, jobs)
	assert.Zero(t, vlans)
	assert.EqualValues(t, 3, otherJobs)
}

func TestDeleteAndGetSwitchReportAMissingOne(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)

	assert.ErrorIs(t, service.Delete(uuid.New()), gorm.ErrRecordNotFound)
	_, err := service.Get(uuid.New())
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestListSwitchesOrdersByName(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	for _, name := range []string{"SW-B", "SW-A"} {
		in := validSwitchInput()
		in.Name = name
		_, err := service.Create(in)
		require.NoError(t, err)
	}

	list, err := service.List()

	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "SW-A", list[0].Name)
}

func TestCreateSwitchPinsTheFingerprintConfirmedInTheForm(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	in := validSwitchInput()
	in.Protocol = models.SwitchProtocolSSH
	in.HostKeyFingerprint = testSSHFingerprint

	sw, err := service.Create(in)

	require.NoError(t, err)
	assert.Equal(t, testSSHFingerprint, sw.HostKeyFingerprint)
}

func TestCreateSwitchRefusesAFingerprintOnTelnet(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	in := validSwitchInput()
	in.HostKeyFingerprint = testSSHFingerprint

	_, err := service.Create(in)

	assert.ErrorIs(t, err, ErrValidation)
}

func TestCreateSwitchRefusesAMalformedFingerprint(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	for _, fingerprint := range []string{
		"SHA256:abc",
		"MD5:bmV3LWtleS1mb3ItdGVzdHMtb25seS0xMjM0NTY3ODk",
		"SHA256:bmV3LWtleS1mb3ItdGVzdHMtb25seS0xMjM0NTY3OD!",
		testSSHFingerprint + "A",
	} {
		in := validSwitchInput()
		in.Protocol = models.SwitchProtocolSSH
		in.HostKeyFingerprint = fingerprint

		_, err := service.Create(in)

		assert.ErrorIs(t, err, ErrValidation, fingerprint)
	}
}

func TestUpdateSwitchRefusesAnEmptyUsername(t *testing.T) {
	service, _ := newSwitchServiceForTest(t)
	sw, err := service.Create(validSwitchInput())
	require.NoError(t, err)

	blank := "   "
	_, err = service.Update(sw.ID, SwitchUpdate{Username: &blank})

	assert.ErrorIs(t, err, ErrValidation)
	stored, err := service.Get(sw.ID)
	require.NoError(t, err)
	assert.Equal(t, "tikman", stored.Username)
}
