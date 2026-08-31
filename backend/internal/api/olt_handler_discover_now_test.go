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

func oltWithJobs(t *testing.T, siteService *services.SiteService, db *gorm.DB) uuid.UUID {
	t.Helper()
	site, err := siteService.Create("Site", "Lokasi", "")
	require.NoError(t, err)

	olt := models.OLT{
		ID: uuid.New(), SiteID: site.ID, Name: "Cariu", IPAddress: "172.30.30.3",
		SNMPCommunity: "public", Username: "admin", Password: "x",
		SSHPort: 22, TelnetPort: 23, SNMPPort: 161,
		PreferredProtocol: models.OLTProtocolTelnet, Status: models.OLTStatusOnline,
	}
	require.NoError(t, db.Create(&olt).Error)
	require.NoError(t, services.NewPollJobService(db).EnsureJobs())
	return olt.ID
}

func TestDiscoverNowSchedulesTheJobAndReturnsAtOnce(t *testing.T) {
	handler, siteService, db := SetupOLTHandlerTest(t)
	oltID := oltWithJobs(t, siteService, db)

	require.NoError(t, db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", oltID, models.PollKindDiscovery).
		Update("due_at", time.Now().Add(5*time.Hour)).Error)

	w, c := SetupTestContext("POST", "/api/olts/"+oltID.String()+"/discover-now", nil)
	c.Params = gin.Params{{Key: "id", Value: oltID.String()}}
	handler.DiscoverNow(c)

	// Accepted, not OK: discovery takes minutes, and the worker runs it. A
	// handler that waited would outlive every timeout between here and the
	// browser.
	assert.Equal(t, http.StatusAccepted, w.Code)

	var job models.OLTPollJob
	require.NoError(t, db.Where("olt_id = ? AND kind = ?", oltID, models.PollKindDiscovery).First(&job).Error)
	assert.False(t, job.DueAt.After(time.Now()))
}

func TestDiscoverNowRefusesWhileAPassIsAlreadyRunning(t *testing.T) {
	handler, siteService, db := SetupOLTHandlerTest(t)
	oltID := oltWithJobs(t, siteService, db)

	require.NoError(t, db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", oltID, models.PollKindDiscovery).
		Updates(map[string]interface{}{"locked_by": "worker-1", "locked_at": time.Now()}).Error)

	w, c := SetupTestContext("POST", "/api/olts/"+oltID.String()+"/discover-now", nil)
	c.Params = gin.Params{{Key: "id", Value: oltID.String()}}
	handler.DiscoverNow(c)

	// The caller is asking for the pass that is already happening; queueing a
	// second one would put two readers on one SNMP agent.
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDiscoverNowRejectsAnOLTThatDoesNotExist(t *testing.T) {
	handler, _, _ := SetupOLTHandlerTest(t)

	missing := uuid.New().String()
	w, c := SetupTestContext("POST", "/api/olts/"+missing+"/discover-now", nil)
	c.Params = gin.Params{{Key: "id", Value: missing}}
	handler.DiscoverNow(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
