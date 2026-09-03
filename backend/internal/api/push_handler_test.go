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
// device token.
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

// tokensStored reads push_subscriptions directly, deliberately bypassing
// PushService.TokensForRoles — that method inner-joins to users, and these
// handler tests authenticate with a bare uuid.New() rather than a real User
// row, exactly to keep them about the handler's own scoping, not about
// TokensForRoles's join (already covered by Task 2's tests, which do create
// real users).
func tokensStored(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tokens []string
	require.NoError(t, db.Model(&models.PushSubscription{}).Pluck("fcm_token", &tokens).Error)
	return tokens
}

func pushRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/push/subscribe", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestPushSubscribeStoresTheCallersToken(t *testing.T) {
	handler, db := setupPushHandler(t)
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodPost, `{"fcm_token":"token-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"token-a"}, tokensStored(t, db))
}

func TestPushSubscribeRejectsAMissingToken(t *testing.T) {
	handler, _ := setupPushHandler(t)
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodPost, `{}`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPushUnsubscribeIsANoOpForATokenTheCallerDoesNotOwn(t *testing.T) {
	handler, db := setupPushHandler(t)
	owner := uuid.New()
	require.NoError(t, services.NewPushService(db).Subscribe(owner, "token-a"))
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodDelete, `{"fcm_token":"token-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code, "unsubscribing someone else's token must not error and reveal it exists")
	assert.Equal(t, []string{"token-a"}, tokensStored(t, db), "the token must still be there")
}

func TestPushUnsubscribeRemovesTheCallersOwnToken(t *testing.T) {
	handler, db := setupPushHandler(t)
	owner := uuid.New()
	require.NoError(t, services.NewPushService(db).Subscribe(owner, "token-a"))
	router := asPushUser(handler, owner)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodDelete, `{"fcm_token":"token-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, tokensStored(t, db))
}
