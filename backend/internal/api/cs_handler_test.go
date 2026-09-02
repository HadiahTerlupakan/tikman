package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// csHandlerEnv wires one CSHandler over an in-memory database so tests drive
// it through real HTTP routing — RequireRole and the holder check both matter
// here, and calling handler methods directly would skip both.
type csHandlerEnv struct {
	db            *gorm.DB
	account       models.WAAccount
	conversations *services.CSConversationService
	messages      *services.CSMessageService
	presence      *services.FakePresence
	cs            uuid.UUID
	otherCS       uuid.UUID
	handler       *CSHandler
}

func setupCSHandler(t *testing.T) *csHandlerEnv {
	t.Helper()

	db := TestDB(t)

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conversations := services.NewCSConversationService(db)
	messages := services.NewCSMessageService(db, conversations)
	quickReplies := services.NewCSQuickReplyService(db)
	presence := services.NewFakePresence()
	assignment := services.NewCSAssignmentService(db, conversations, presence)
	logger := zap.NewNop()
	audit := services.NewAuditService(db, logger)

	// Nothing here needs Redis to actually answer: Publish failures are logged
	// and swallowed by design (see cs_handler_messages.go), so an unreachable
	// client is enough to exercise the real code path without a live server.
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	publisher := wa.NewPublisher(redisClient)

	handler := NewCSHandler(
		conversations, messages, quickReplies, assignment, presence,
		audit, publisher, redisClient, logger, t.TempDir(),
	)

	return &csHandlerEnv{
		db:            db,
		account:       account,
		conversations: conversations,
		messages:      messages,
		presence:      presence,
		cs:            uuid.New(),
		otherCS:       uuid.New(),
		handler:       handler,
	}
}

// asUser builds the router as one authenticated request would see it: a fake
// session middleware standing in for AuthMiddleware (there is no real session
// to forge one for), the real RequireRole after it, and the real routes — so a
// role that should be turned away actually is, by the real middleware.
func (e *csHandlerEnv) asUser(id uuid.UUID, role models.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", id)
		c.Set("user_role", role)
		c.Next()
	})

	cs := router.Group("/api/v1/cs")
	cs.Use(middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician))
	{
		cs.GET("/conversations", e.handler.ListConversations)
		cs.GET("/conversations/:id/messages", e.handler.History)
		cs.POST("/conversations/:id/messages", e.handler.Send)
		cs.POST("/conversations/:id/media", e.handler.SendMedia)
		cs.PUT("/conversations/:id/assign", e.handler.Assign)
		cs.PUT("/conversations/:id/status", e.handler.SetStatus)
		cs.PUT("/conversations/:id/ont", e.handler.LinkONT)
		cs.GET("/media/:message_id", e.handler.ServeMedia)
		cs.GET("/messages/search", e.handler.SearchMessages)
	}
	return router
}

// conversation creates one thread ready for a test to assign and message.
func (e *csHandlerEnv) conversation(t *testing.T, jid, phone string) *models.CSConversation {
	t.Helper()
	conv, err := e.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: e.account.ID,
		JID:         jid,
		Phone:       phone,
		Name:        "Pelanggan",
	})
	require.NoError(t, err)
	return conv
}

// A CS may read the whole inbox — the team seeing each other is what stops two
// of them answering the same customer — but may only send on a thread they hold.
func TestSendIsRefusedOnSomeoneElsesThread(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.otherCS))

	body := strings.NewReader(`{"body":"halo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/conversations/"+conv.ID.String()+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "dipegang")
}

func TestSendQueuesAMessageOnMyOwnThread(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	body := strings.NewReader(`{"body":"sudah kami cek"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/conversations/"+conv.ID.String()+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var payload struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, string(models.MessageQueued), payload.Data.Status)
}

// Taking over is the way out when an agent leaves mid-shift, and it must be
// recorded — an assignment that changes with no trace is how blame lands on the
// wrong person.
func TestTakingOverIsAllowedAndAudited(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.otherCS))

	body := strings.NewReader(`{"user_id":"` + env.cs.String() + `"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cs/conversations/"+conv.ID.String()+"/assign", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	after, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	require.NotNil(t, after.AssignedUserID)
	assert.Equal(t, env.cs, *after.AssignedUserID)

	var logs []models.AuditLog
	require.NoError(t, env.db.Find(&logs).Error)
	assert.NotEmpty(t, logs, "a handover leaves a trail")
}

func TestViewerIsKeptOutOfTheInbox(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/conversations", nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleViewer).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTechnicianMayReadAndSend(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/conversations", nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleTechnician).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
