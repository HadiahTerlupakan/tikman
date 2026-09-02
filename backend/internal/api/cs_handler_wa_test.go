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

// Listing accounts and cutting one off are the same admin-only decision as
// connecting one — this covers the two routes the connect test above does not.
func TestOnlyAdminMayListOrDisconnectAccounts(t *testing.T) {
	env := setupCSHandler(t)

	requests := map[string]*http.Request{
		"list":       httptest.NewRequest(http.MethodGet, "/api/v1/cs/wa-accounts", nil),
		"disconnect": httptest.NewRequest(http.MethodPost, "/api/v1/cs/wa-accounts/"+env.account.ID.String()+"/disconnect", nil),
	}

	for name, req := range requests {
		for _, role := range []models.UserRole{models.UserRoleCS, models.UserRoleTechnician, models.UserRoleViewer} {
			rec := httptest.NewRecorder()
			env.asUser(uuid.New(), role).ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code, name+" "+string(role))
		}
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
