package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func (e *csGroupEnv) leaveGroup(t *testing.T) {
	t.Helper()
	require.NoError(t, e.groups.MarkLeft(e.account.ID, e.group.JID))
}

func TestAnAdminDeletesALeftGroupAndTheDeletionIsAudited(t *testing.T) {
	env := setupCSGroups(t)
	env.leaveGroup(t)

	rec := env.do(t, env.admin, models.UserRoleAdmin, http.MethodDelete, "/groups/"+env.group.ID.String(), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, err := env.groups.Get(env.group.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	var entry models.AuditLog
	require.NoError(t, env.db.Where("resource_type = ? AND action = ?", "wa_group", "delete").First(&entry).Error)
	assert.Equal(t, env.group.ID, *entry.ResourceID)
	assert.Contains(t, string(entry.OldValue), "Teknisi")
}

func TestDeletingAGroupTheNumberIsStillInIsAConflict(t *testing.T) {
	env := setupCSGroups(t)

	rec := env.do(t, env.admin, models.UserRoleAdmin, http.MethodDelete, "/groups/"+env.group.ID.String(), "")

	require.Equal(t, http.StatusConflict, rec.Code)
	var body ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "GROUP_NOT_LEFT", body.Code)
	assert.Equal(t, "Grup hanya bisa dihapus setelah nomor ini keluar dari grup", body.Error)
	_, err := env.groups.Get(env.group.ID)
	assert.NoError(t, err)
}

func TestDeletingAMissingGroupIsNotFound(t *testing.T) {
	env := setupCSGroups(t)

	rec := env.do(t, env.admin, models.UserRoleAdmin, http.MethodDelete, "/groups/"+uuid.NewString(), "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Checked through the route tree production builds, so a gate dropped from
// router.go fails here; a 403 rather than a 404 also proves the route exists.
func TestDeletingAGroupIsClosedToEveryoneButAnAdmin(t *testing.T) {
	path := "/api/v1/cs/groups/" + uuid.NewString()
	for _, role := range []models.UserRole{models.UserRoleCS, models.UserRoleTechnician, models.UserRoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			router, store := rbacTestRouter(t)
			assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, http.MethodDelete, path))
		})
	}
}

func TestALeftGroupSurvivesANonAdminDelete(t *testing.T) {
	env := setupCSGroups(t)
	env.leaveGroup(t)

	rec := env.do(t, env.cs, models.UserRoleCS, http.MethodDelete, "/groups/"+env.group.ID.String(), "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	_, err := env.groups.Get(env.group.ID)
	assert.NoError(t, err)
}
