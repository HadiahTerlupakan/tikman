package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
