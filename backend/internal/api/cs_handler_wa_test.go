package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Pairing a number, and cutting it off, are admin decisions: a CS doing either
// by accident takes the whole team off WhatsApp.
func TestOnlyAdminMayConnectANumber(t *testing.T) {
	env := setupCSHandler(t)

	for role, want := range map[models.UserRole]int{
		models.UserRoleCS:         http.StatusForbidden,
		models.UserRoleTechnician: http.StatusForbidden,
		models.UserRoleViewer:     http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/cs/wa-accounts/"+env.account.ID.String()+"/connect", nil)
		rec := httptest.NewRecorder()
		env.asUser(uuid.New(), role).ServeHTTP(rec, req)
		assert.Equal(t, want, rec.Code, string(role))
	}
}

// Cutting a number off is the same admin-only decision as connecting one —
// this covers the one route the connect test above does not.
func TestOnlyAdminMayDisconnectAnAccount(t *testing.T) {
	env := setupCSHandler(t)

	for _, role := range []models.UserRole{models.UserRoleCS, models.UserRoleTechnician, models.UserRoleViewer} {
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/cs/wa-accounts/"+env.account.ID.String()+"/disconnect", nil)
		rec := httptest.NewRecorder()
		env.asUser(uuid.New(), role).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, string(role))
	}
}

// Reading the account list is not the same decision as changing it: a CS or
// technician who cannot see whether the number is connected has no way to
// know their replies are not going out. Viewer stays out — the whole /cs
// group is closed to that role, this is not a wa-accounts-specific check.
func TestCSAndTechnicianMayListAccountsButViewerMayNot(t *testing.T) {
	env := setupCSHandler(t)

	for role, want := range map[models.UserRole]int{
		models.UserRoleCS:         http.StatusOK,
		models.UserRoleTechnician: http.StatusOK,
		models.UserRoleViewer:     http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/wa-accounts", nil)
		rec := httptest.NewRecorder()
		env.asUser(uuid.New(), role).ServeHTTP(rec, req)
		assert.Equal(t, want, rec.Code, string(role))
	}
}

// The account is marked pairing, and the response says so, before the wa
// process has answered at all — a browser polling the list must not have to
// wait on a process it cannot see.
func TestConnectMarksTheAccountPairingAndAccepts(t *testing.T) {
	env := setupCSHandler(t)

	body := strings.NewReader(`{"phone":"628111222333"}`)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/cs/wa-accounts/"+env.account.ID.String()+"/connect", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleAdmin).ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)

	var payload struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, string(models.WAAccountPairing), payload.Data.Status)

	var row models.WAAccount
	require.NoError(t, env.db.First(&row, "id = ?", env.account.ID).Error)
	assert.Equal(t, models.WAAccountPairing, row.Status)
}

// PairPhone on the wa side rejects a number written with a leading zero, so
// Connect must normalize before the control message is even built — not
// leave it to fail on the other side of Redis.
func TestConnectNormalizesA0PrefixedPhoneBeforePublishing(t *testing.T) {
	msg, err := connectControlMessage(uuid.New(), "081112223334")

	require.NoError(t, err)
	assert.Equal(t, "6281112223334", msg.Phone)
}

func TestOnlyAdminMayChangeQuickReplies(t *testing.T) {
	env := setupCSHandler(t)

	body := strings.NewReader(`{"title":"Cek LOS","body":"Mohon cek lampu LOS."}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/quick-replies", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// The service answers gorm.ErrRecordNotFound for a template that is not there,
// and the handler must turn that into a 404. A 500 would send an admin hunting
// a fault that does not exist.
func TestUpdatingAQuickReplyThatIsNotThereAnswers404(t *testing.T) {
	env := setupCSHandler(t)

	body := strings.NewReader(`{"title":"Cek LOS","body":"Mohon cek lampu LOS."}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cs/quick-replies/"+uuid.New().String(), body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleAdmin).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDeletingAQuickReplyThatIsNotThereAnswers404(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cs/quick-replies/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleAdmin).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Reading them is not an admin matter: a CS who cannot read the templates
// cannot use them.
func TestAnyoneInTheInboxMayReadQuickReplies(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/quick-replies", nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
