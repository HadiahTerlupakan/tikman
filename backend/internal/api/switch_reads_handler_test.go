package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

func TestSwitchReadRoutesFollowTheRoles(t *testing.T) {
	router, store := rbacTestRouter(t)
	base := "/api/v1/switches/" + uuid.New().String()
	reads := []string{"/interfaces", "/vlans", "/neighbors", "/backups", "/backups/" + uuid.NewString(),
		"/backups/diff?from=" + uuid.NewString() + "&to=" + uuid.NewString()}
	allRoles := []models.UserRole{models.UserRoleViewer, models.UserRoleCS, models.UserRoleTechnician, models.UserRoleAdmin}

	for _, role := range allRoles {
		for _, path := range reads {
			assert.Equal(t, http.StatusNotFound, requestAs(t, router, store, role, http.MethodGet, base+path), "%s %s", role, path)
		}
	}
	for _, path := range []string{"/refresh", "/backup"} {
		for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS} {
			assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, http.MethodPost, base+path), "%s %s", role, path)
		}
		for _, role := range []models.UserRole{models.UserRoleTechnician, models.UserRoleAdmin} {
			assert.Equal(t, http.StatusNotFound, requestAs(t, router, store, role, http.MethodPost, base+path), "%s %s", role, path)
		}
	}
	for _, path := range []string{base + "/trust-host-key", "/api/v1/switches/test-connection"} {
		for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS, models.UserRoleTechnician} {
			assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, http.MethodPost, path), "%s %s", role, path)
		}
	}
	assert.Equal(t, http.StatusNotFound, requestAs(t, router, store, models.UserRoleAdmin, http.MethodPost, base+"/trust-host-key"))
	assert.Equal(t, http.StatusBadRequest, requestAs(t, router, store, models.UserRoleAdmin, http.MethodPost, "/api/v1/switches/test-connection"))
}

type switchReadsFixture struct {
	handler *SwitchReadsHandler
	jobs    *services.SwitchPollJobService
	db      *gorm.DB
	sw      *models.Switch
}

func newSwitchReadsFixture(t *testing.T, protocol models.SwitchProtocol) switchReadsFixture {
	t.Helper()
	db := TestDB(t)
	switches := services.NewSwitchService(db, testEncryptionKey)
	jobs := services.NewSwitchPollJobService(db)
	sw, err := switches.Create(services.SwitchInput{Name: "SW-TEST", Host: "192.0.2.10", Vendor: models.SwitchVendorHuaweiCE,
		SNMPCommunity: "c", Protocol: protocol, Username: "tikman", Password: "p"})
	require.NoError(t, err)
	handler := NewSwitchReadsHandler(switches, services.NewSwitchQueryService(db), jobs, services.NewSwitchProber(), nil)
	return switchReadsFixture{handler: handler, jobs: jobs, db: db, sw: sw}
}

func (f switchReadsFixture) ctx(method string, params gin.Params) (*gin.Context, func() (int, string)) {
	w, c := SetupTestContext(method, "/api/v1/switches", nil)
	c.Params = params
	return c, func() (int, string) { return w.Code, w.Body.String() }
}

func TestTrustHostKeyPinsThePresentedFingerprintAndClearsTheFlag(t *testing.T) {
	f := newSwitchReadsFixture(t, models.SwitchProtocolSSH)
	require.NoError(t, f.db.Model(f.sw).Updates(map[string]interface{}{
		"presented_fingerprint": "SHA256:bmV3LWtleS1mb3ItdGVzdHMtb25seS0xMjM0NTY3ODk", "host_key_changed": true}).Error)
	c, result := f.ctx(http.MethodPost, gin.Params{{Key: "id", Value: f.sw.ID.String()}})

	f.handler.TrustHostKey(c)

	code, body := result()
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"host_key_fingerprint":"SHA256:bmV3LWtleS1mb3ItdGVzdHMtb25seS0xMjM0NTY3ODk"`)
	assert.Contains(t, body, `"host_key_changed":false`)
}

func TestTrustHostKeyRefusesWhenNothingWasPresented(t *testing.T) {
	f := newSwitchReadsFixture(t, models.SwitchProtocolSSH)
	c, result := f.ctx(http.MethodPost, gin.Params{{Key: "id", Value: f.sw.ID.String()}})

	f.handler.TrustHostKey(c)

	code, _ := result()
	assert.Equal(t, http.StatusBadRequest, code)
}

func dueAt(t *testing.T, f switchReadsFixture, kind models.SwitchPollKind) time.Time {
	t.Helper()
	var job models.SwitchPollJob
	require.NoError(t, f.db.First(&job, "switch_id = ? AND kind = ?", f.sw.ID, kind).Error)
	return job.DueAt
}

func TestBackupNowMakesOnlyTheBackupJobDue(t *testing.T) {
	f := newSwitchReadsFixture(t, models.SwitchProtocolTelnet)
	later := time.Now().Add(time.Hour)
	require.NoError(t, f.db.Model(&models.SwitchPollJob{}).Where("switch_id = ?", f.sw.ID).Update("due_at", later).Error)
	c, result := f.ctx(http.MethodPost, gin.Params{{Key: "id", Value: f.sw.ID.String()}})

	f.handler.BackupNow(c)

	code, _ := result()
	require.Equal(t, http.StatusAccepted, code)
	assert.True(t, dueAt(t, f, models.SwitchPollKindBackup).Before(later))
	assert.WithinDuration(t, later, dueAt(t, f, models.SwitchPollKindInventory), time.Second)
}

func TestBackupDiffAndBackupTextOverHTTP(t *testing.T) {
	f := newSwitchReadsFixture(t, models.SwitchProtocolTelnet)
	now := time.Now()
	from := models.SwitchConfigBackup{SwitchID: f.sw.ID, TakenAt: now.Add(-time.Hour), Reason: "daily", RawSHA256: "a", Masked: "vlan 10\n", Size: 8}
	to := models.SwitchConfigBackup{SwitchID: f.sw.ID, TakenAt: now, Reason: "daily", RawSHA256: "b", Masked: "vlan 20\n", Size: 8}
	require.NoError(t, f.db.Create(&from).Error)
	require.NoError(t, f.db.Create(&to).Error)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/switches/x/backups/diff?from="+from.ID.String()+"&to="+to.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: f.sw.ID.String()}}
	f.handler.BackupDiff(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `-vlan 10\n+vlan 20\n`)

	w, c = SetupTestContext(http.MethodGet, "/api/v1/switches/x/backups/diff?from=bad&to=bad", nil)
	c.Params = gin.Params{{Key: "id", Value: f.sw.ID.String()}}
	f.handler.BackupDiff(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, result := f.ctx(http.MethodGet, gin.Params{{Key: "id", Value: f.sw.ID.String()}, {Key: "bid", Value: to.ID.String()}})
	f.handler.Backup(c)
	code, body := result()
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `"masked":"vlan 20\n"`)
}
