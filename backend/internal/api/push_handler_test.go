package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// asPushUser builds the router as one authenticated request would see it —
// the same fake-session-then-real-routes shape cs_handler_test.go's asUser
// uses, minus RequireRole, since any logged-in role may manage its own
// installation ID.
func asPushUser(handler *PushHandler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.POST("/api/v1/push/subscribe", handler.Subscribe)
	router.DELETE("/api/v1/push/subscribe", handler.Unsubscribe)
	return router
}

func setupPushHandler(t *testing.T) (*PushHandler, *gorm.DB) {
	t.Helper()
	db := TestDB(t)
	return NewPushHandler(services.NewPushService(db)), db
}

// fidsStored reads push_subscriptions directly, deliberately bypassing
// PushService.FIDsForRoles — that method inner-joins to users, and these
// handler tests authenticate with a bare uuid.New() rather than a real User
// row, exactly to keep them about the handler's own scoping, not about
// FIDsForRoles's join (already covered by Task 2's tests, which do create
// real users).
func fidsStored(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var fids []string
	require.NoError(t, db.Model(&models.PushSubscription{}).Pluck("fid", &fids).Error)
	return fids
}

func pushRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/push/subscribe", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestPushSubscribeStoresTheCallersFID(t *testing.T) {
	handler, db := setupPushHandler(t)
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodPost, `{"fid":"fid-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"fid-a"}, fidsStored(t, db))
}

func TestPushSubscribeRejectsAMissingFID(t *testing.T) {
	handler, _ := setupPushHandler(t)
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodPost, `{}`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPushUnsubscribeIsANoOpForAnFIDTheCallerDoesNotOwn(t *testing.T) {
	handler, db := setupPushHandler(t)
	owner := uuid.New()
	require.NoError(t, services.NewPushService(db).Subscribe(owner, "fid-a"))
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodDelete, `{"fid":"fid-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code, "unsubscribing someone else's FID must not error and reveal it exists")
	assert.Equal(t, []string{"fid-a"}, fidsStored(t, db), "the FID must still be there")
}

func TestPushUnsubscribeRemovesTheCallersOwnFID(t *testing.T) {
	handler, db := setupPushHandler(t)
	owner := uuid.New()
	require.NoError(t, services.NewPushService(db).Subscribe(owner, "fid-a"))
	router := asPushUser(handler, owner)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodDelete, `{"fid":"fid-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, fidsStored(t, db))
}
