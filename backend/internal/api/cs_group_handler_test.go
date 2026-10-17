package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
)

type csGroupEnv struct {
	*csHandlerEnv
	groups   *services.WAGroupService
	messages *services.WAGroupMessageService
	group    *models.WAGroup
	admin    uuid.UUID
}

func setupCSGroups(t *testing.T) *csGroupEnv {
	t.Helper()
	inbox := setupCSHandler(t)
	groups := services.NewWAGroupService(inbox.db)
	group, err := groups.Upsert(inbox.account.ID, services.GroupSnapshot{JID: "120363000000000001@g.us", Name: "Teknisi"})
	require.NoError(t, err)
	return &csGroupEnv{
		csHandlerEnv: inbox,
		groups:       groups,
		messages:     services.NewWAGroupMessageService(inbox.db),
		group:        group,
		admin:        csTestUser(t, inbox.db, "Admin Satu", "AS"),
	}
}

// router registers the real group routes, so the RBAC under test is the one
// production uses rather than a copy of it.
func (e *csGroupEnv) router(id uuid.UUID, role models.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", id)
		c.Set("user_role", role)
		c.Next()
	})
	cs := router.Group("/api/v1/cs")
	cs.Use(middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician))
	h := &handlers{csGroupHandler: NewCSGroupHandler(e.handler, e.groups, e.messages,
		services.NewWAGroupActionService(e.db), zap.NewNop())}
	h.registerCSGroupRoutes(cs)
	return router
}

func (e *csGroupEnv) do(t *testing.T, id uuid.UUID, role models.UserRole, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/cs"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.router(id, role).ServeHTTP(rec, req)
	return rec
}

func TestReadingAGroupsHistoryClearsItsUnreadCount(t *testing.T) {
	env := setupCSGroups(t)
	_, _, err := env.messages.SaveArrived(services.GroupArrival{
		GroupID: env.group.ID, WAMessageID: "W1", Direction: models.MessageIn,
		SenderName: "Budi", Kind: models.MessageKindText, Body: "ODP putus", At: time.Now(),
	})
	require.NoError(t, err)

	rec := env.do(t, env.cs, models.UserRoleCS, http.MethodGet, "/groups/"+env.group.ID.String()+"/messages", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ODP putus")
	group, err := env.groups.Get(env.group.ID)
	require.NoError(t, err)
	assert.Zero(t, group.UnreadCount)
}

func TestACSSendsToAGroupSignedAndUnderTheirName(t *testing.T) {
	env := setupCSGroups(t)

	rec := env.do(t, env.cs, models.UserRoleCS, http.MethodPost,
		"/groups/"+env.group.ID.String()+"/messages", `{"body":"OTW ke lokasi"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var answer struct {
		Data models.WAGroupMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	assert.Equal(t, models.MessageQueued, answer.Data.Status)
	assert.Equal(t, "Budi Santoso", answer.Data.SenderName)
	assert.Contains(t, answer.Data.Body, "BS", "group replies carry the same signature as inbox replies")

	var inbox int64
	require.NoError(t, env.db.Model(&models.CSMessage{}).Count(&inbox).Error)
	assert.Zero(t, inbox, "nothing sent to a group lands in the customer inbox")
}

func TestSendingIsRefusedWhereTheNumberCannotPost(t *testing.T) {
	cases := []struct {
		name string
		snap services.GroupSnapshot
		left bool
		code string
	}{
		{"a group the number left", services.GroupSnapshot{JID: "120363000000000001@g.us"}, true, "GROUP_LEFT"},
		{"an admins-only group it does not run", services.GroupSnapshot{JID: "120363000000000001@g.us", Announce: true}, false, "GROUP_ANNOUNCE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupCSGroups(t)
			_, err := env.groups.Upsert(env.account.ID, tc.snap)
			require.NoError(t, err)
			if tc.left {
				require.NoError(t, env.groups.MarkLeft(env.account.ID, tc.snap.JID))
			}

			rec := env.do(t, env.cs, models.UserRoleCS, http.MethodPost,
				"/groups/"+env.group.ID.String()+"/messages", `{"body":"halo"}`)

			assert.Equal(t, http.StatusConflict, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.code)
			var queued int64
			require.NoError(t, env.db.Model(&models.WAGroupMessage{}).Count(&queued).Error)
			assert.Zero(t, queued)
		})
	}
}

// Managing a group speaks for the number to everyone in it: reading and
// writing are the team's, changing the group is an admin's.
func TestOnlyAnAdminMayManageAGroup(t *testing.T) {
	env := setupCSGroups(t)
	path := "/groups/" + env.group.ID.String() + "/actions"
	body := `{"action":"remove","participants":["081234567890"]}`

	refused := env.do(t, env.cs, models.UserRoleCS, http.MethodPost, path, body)
	assert.Equal(t, http.StatusForbidden, refused.Code)

	accepted := env.do(t, env.admin, models.UserRoleAdmin, http.MethodPost, path, body)
	require.Equal(t, http.StatusAccepted, accepted.Code, accepted.Body.String())
	var answer struct {
		Data models.WAGroupAction `json:"data"`
	}
	require.NoError(t, json.Unmarshal(accepted.Body.Bytes(), &answer))
	assert.Equal(t, models.GroupActionQueued, answer.Data.Status)

	status := env.do(t, env.cs, models.UserRoleCS, http.MethodGet, "/group-actions/"+answer.Data.ID.String(), "")
	assert.Equal(t, http.StatusOK, status.Code, "anyone on the team may see how a request went")
}

func TestCreatingAGroupWithANameWhatsAppRefusesIsA400(t *testing.T) {
	env := setupCSGroups(t)
	body := `{"wa_account_id":"` + env.account.ID.String() +
		`","name":"Nama grup yang jauh terlalu panjang","participants":["081234567890"]}`

	rec := env.do(t, env.admin, models.UserRoleAdmin, http.MethodPost, "/groups", body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "25 karakter")
}

func TestAViewerCannotReadTheGroups(t *testing.T) {
	env := setupCSGroups(t)

	rec := env.do(t, uuid.New(), models.UserRoleViewer, http.MethodGet, "/groups", "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
