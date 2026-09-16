package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// reportDay is the report for 10 September 2026 in WIB.
const reportDay = "from=2026-09-10&to=2026-09-10"

// performanceRouter builds the report routes as one authenticated request sees
// them: a stand-in for AuthMiddleware, then the real RequireRole.
func performanceRouter(db *gorm.DB, userID uuid.UUID, role models.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Next()
	})
	handler := NewCSPerformanceHandler(services.NewCSPerformanceService(db))
	cs := router.Group("/api/v1/cs")
	cs.Use(middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician))
	cs.GET("/performance/summary", handler.Summary)
	cs.GET("/performance/waits", handler.Waits)
	return router
}

// plantReply leaves a wait answered by one CS at 10:00 WIB on 10 September.
func plantReply(t *testing.T, db *gorm.DB, by uuid.UUID) {
	t.Helper()
	started := time.Date(2026, 9, 10, 2, 55, 0, 0, time.UTC)
	ended := time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC)
	reason, message := models.WaitReplied, uuid.New()
	require.NoError(t, db.Create(&models.CSWait{
		ConversationID: uuid.New(), WAAccountID: uuid.New(),
		StartedAt: started, CustomerSentAt: started, LastCustomerSentAt: started,
		EndedAt: &ended, EndReason: &reason, EndedBy: &by, ReplyMessageID: &message,
	}).Error)
}

func getJSON(t *testing.T, router *gin.Engine, path string, into any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code == http.StatusOK && into != nil {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), into))
	}
	return rec.Code
}

type summaryBody struct {
	Data struct {
		TargetMinutes int `json:"target_minutes"`
		Team          struct {
			Replies struct {
				Count int `json:"count"`
			} `json:"replies"`
		} `json:"team"`
		Agents []struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"agents"`
	} `json:"data"`
}

type waitsBody struct {
	Data []struct {
		EndedBy *uuid.UUID `json:"ended_by"`
	} `json:"data"`
	Total int64 `json:"total"`
}

func TestAnAdminSeesEveryCSInTheSummary(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body summaryBody
	code := getJSON(t, performanceRouter(db, uuid.New(), models.UserRoleAdmin),
		"/api/v1/cs/performance/summary?"+reportDay, &body)

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 15, body.Data.TargetMinutes)
	assert.Equal(t, 2, body.Data.Team.Replies.Count)
	assert.Len(t, body.Data.Agents, 2)
}

// A CS is shown the team's figures, because that is what they are part of, and
// their own row, because the others' are not theirs to read.
func TestACSSeesTheTeamButOnlyTheirOwnRow(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body summaryBody
	code := getJSON(t, performanceRouter(db, ani, models.UserRoleCS),
		"/api/v1/cs/performance/summary?"+reportDay, &body)

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 2, body.Data.Team.Replies.Count)
	require.Len(t, body.Data.Agents, 1)
	assert.Equal(t, ani, body.Data.Agents[0].UserID)
}

// Naming somebody else in the query must not hand their waits over.
func TestACSCannotListAnotherCSsWaits(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body waitsBody
	code := getJSON(t, performanceRouter(db, ani, models.UserRoleCS),
		"/api/v1/cs/performance/waits?"+reportDay+"&user_id="+budi.String(), &body)

	require.Equal(t, http.StatusOK, code)
	assert.EqualValues(t, 1, body.Total)
	require.Len(t, body.Data, 1)
	assert.Equal(t, &ani, body.Data[0].EndedBy)
}

func TestAnAdminCanListOneCSsWaits(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body waitsBody
	code := getJSON(t, performanceRouter(db, uuid.New(), models.UserRoleAdmin),
		"/api/v1/cs/performance/waits?"+reportDay+"&user_id="+budi.String(), &body)

	require.Equal(t, http.StatusOK, code)
	require.Len(t, body.Data, 1)
	assert.Equal(t, &budi, body.Data[0].EndedBy)
}

func TestAViewerIsTurnedAwayFromTheReport(t *testing.T) {
	db := TestDB(t)
	router := performanceRouter(db, uuid.New(), models.UserRoleViewer)

	assert.Equal(t, http.StatusForbidden, getJSON(t, router, "/api/v1/cs/performance/summary?"+reportDay, nil))
	assert.Equal(t, http.StatusForbidden, getJSON(t, router, "/api/v1/cs/performance/waits?"+reportDay, nil))
}

func TestTheReportRefusesARangeItCannotRead(t *testing.T) {
	db := TestDB(t)
	router := performanceRouter(db, uuid.New(), models.UserRoleAdmin)
	cases := map[string]string{
		"not a date":            "from=2026-9-10&to=2026-09-10",
		"nothing at all":        "",
		"ends before it starts": "from=2026-09-11&to=2026-09-10",
		"longer than a year":    "from=2026-01-01&to=2027-01-02",
	}
	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusBadRequest,
				getJSON(t, router, "/api/v1/cs/performance/summary?"+query, nil))
		})
	}
}

func TestTheWaitListRefusesAUserIDItCannotRead(t *testing.T) {
	db := TestDB(t)

	code := getJSON(t, performanceRouter(db, uuid.New(), models.UserRoleAdmin),
		"/api/v1/cs/performance/waits?"+reportDay+"&user_id=bukan-uuid", nil)

	assert.Equal(t, http.StatusBadRequest, code)
}
