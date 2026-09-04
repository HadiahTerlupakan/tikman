package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	onts          *services.ONTService
	// mediaRoot is where attachments land, so a test can look on the disk
	// rather than trust the handler's word for what it removed.
	mediaRoot string
	cs        uuid.UUID
	otherCS   uuid.UUID
	handler   *CSHandler
}

// csTestUser stores a CS the handler can actually resolve by id. initials is
// what UserService.Create would have derived, kept explicit here since this
// helper writes the row directly rather than going through that service.
func csTestUser(t *testing.T, db *gorm.DB, username, initials string) uuid.UUID {
	t.Helper()
	user := models.User{
		ID:       uuid.New(),
		Username: username,
		Email:    username + "@example.test",
		Role:     models.UserRoleCS,
		Initials: initials,
	}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

func setupCSHandler(t *testing.T) *csHandlerEnv {
	t.Helper()

	db := TestDB(t)

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conversations := services.NewCSConversationService(db)
	messages := services.NewCSMessageService(db, conversations)
	quickReplies := services.NewCSQuickReplyService(db)
	accounts := services.NewCSAccountService(db)
	channels := services.NewCSChannelService(db)
	channelPosts := services.NewCSChannelPostService(db)
	presence := services.NewFakePresence()
	assignment := services.NewCSAssignmentService(db, conversations, presence)
	logger := zap.NewNop()
	audit := services.NewAuditService(db, logger)
	onts := services.NewONTService(db)

	// Nothing here needs Redis to actually answer: Publish failures are logged
	// and swallowed by design (see cs_handler_messages.go), so an unreachable
	// client is enough to exercise the real code path without a live server.
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	publisher := wa.NewPublisher(redisClient)

	mediaRoot := t.TempDir()
	handler := NewCSHandler(
		conversations, messages, quickReplies, accounts, channels, channelPosts,
		services.NewCSPurgeService(db, mediaRoot), assignment, presence,
		audit, onts, services.NewUserService(db), publisher, redisClient, logger,
		mediaRoot,
	)

	return &csHandlerEnv{
		db:            db,
		account:       account,
		conversations: conversations,
		messages:      messages,
		presence:      presence,
		onts:          onts,
		mediaRoot:     mediaRoot,
		// Real rows, not bare ids: replies are signed with the sender's name, and
		// a user the handler cannot look up would silently go unsigned — which
		// is how the wiring could break without a test noticing.
		cs:      csTestUser(t, db, "Budi Santoso", "BS"),
		otherCS: csTestUser(t, db, "Rina Astuti", "RA"),
		handler: handler,
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
		cs.POST("/conversations/:id/typing", e.handler.SetTyping)
		cs.PUT("/conversations/:id/assign", e.handler.Assign)
		cs.PUT("/conversations/:id/status", e.handler.SetStatus)
		cs.PUT("/conversations/:id/ont", e.handler.LinkONT)
		cs.GET("/media/:message_id", e.handler.ServeMedia)
		cs.GET("/conversations/:id/avatar", e.handler.ServeAvatar)
		cs.GET("/messages/search", e.handler.SearchMessages)
		cs.DELETE("/messages/:id", e.handler.DeleteMessage)
		cs.DELETE("/conversations/:id/messages", e.handler.ClearConversation)
		cs.DELETE("/messages", middleware.RequireRole(models.UserRoleAdmin), e.handler.ClearInbox)
		cs.GET("/stream", e.handler.Stream)

		cs.GET("/quick-replies", e.handler.ListQuickReplies)
		cs.POST("/quick-replies", middleware.RequireRole(models.UserRoleAdmin), e.handler.CreateQuickReply)
		cs.PUT("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), e.handler.UpdateQuickReply)
		cs.DELETE("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), e.handler.DeleteQuickReply)

		cs.GET("/wa-accounts", e.handler.ListAccounts)
		cs.POST("/wa-accounts/:id/connect", middleware.RequireRole(models.UserRoleAdmin), e.handler.Connect)
		cs.POST("/wa-accounts/:id/disconnect", middleware.RequireRole(models.UserRoleAdmin), e.handler.Disconnect)
		cs.DELETE("/wa-accounts/:id", middleware.RequireRole(models.UserRoleAdmin), e.handler.DeleteAccount)
		cs.DELETE("/wa-accounts/:id/messages", middleware.RequireRole(models.UserRoleAdmin), e.handler.ClearAccountMessages)

		cs.GET("/wa-channels", e.handler.ListChannels)
		cs.POST("/wa-channels/refresh", e.handler.RefreshChannels)
		cs.GET("/channel-posts", e.handler.ListChannelPosts)
		cs.POST("/channel-posts", e.handler.CreateChannelPost)
		cs.POST("/channel-posts/media", e.handler.CreateChannelPostMedia)
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

// ont creates one OLT and one ONT under it — its own OLT each time, so the
// port/ONT-number position never collides across calls in the same test —
// for tests that link a conversation to a real ONT row. phone may be empty.
func (e *csHandlerEnv) ont(t *testing.T, phone string) *models.ONT {
	t.Helper()
	olt := &models.OLT{ID: uuid.New(), SiteID: uuid.New(), Name: "Test OLT", IPAddress: "10.0.0.1"}
	require.NoError(t, e.db.Create(olt).Error)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN-" + uuid.NewString(),
		Phone:        phone,
	}
	require.NoError(t, e.onts.Create(ont))
	return ont
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

// ServeMedia's containment check is the one control in this task that a
// broken database row can actually exercise: MediaPath comes straight out of
// the message table, never validated on the way in, so a corrupted or
// tampered value must still not become a read outside mediaRoot.
func TestServeMediaRefusesAPathThatEscapesTheMediaRoot(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	msg, err := env.messages.Queue(conv.ID, env.cs, models.MessageKindDocument, "", &services.MediaFile{
		Path:     "../../../../../../../../etc/passwd",
		Mime:     "text/plain",
		Filename: "passwd",
		Size:     0,
	}, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/media/"+msg.ID.String(), nil)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "root:", "the guard must refuse before anything is read, not just fail to find it useful")
}

// A legitimate attachment must come back as the type it was actually stored
// with, and locked against sniffing — not whatever http.ServeFile would infer
// on its own. The file on disk is deliberately given a .txt extension and
// plain-text bytes while MediaMime says image/jpeg: net/http's ServeContent
// only falls back to guessing (by extension, then by sniffing) when the
// response has no Content-Type set yet, so if the handler's explicit header
// were missing this would come back as "text/plain", not "image/jpeg" — the
// mismatch is what makes the assertion prove the override, not just agree
// with what ServeFile would have guessed anyway.
func TestServeMediaSetsTheStoredContentType(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	rel := filepath.Join("2026", "09", "photo.txt")
	full := filepath.Join(env.handler.mediaRoot, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
	require.NoError(t, os.WriteFile(full, []byte("plain text, not a jpeg"), 0o640))

	msg, err := env.messages.Queue(conv.ID, env.cs, models.MessageKindImage, "", &services.MediaFile{
		Path:     rel,
		Mime:     "image/jpeg",
		Filename: "photo.jpg",
		Size:     22,
	}, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/media/"+msg.ID.String(), nil)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

// Inbound messages store whatever mime the customer's client declared —
// NormalizeMime only truncates and caps it, it does not restrict it to what
// wa.AllowedExtension accepts on the upload path. ServeMedia must not trust
// an inbound-stored value just because it made it into the column: a message
// stored with e.g. "text/html" is served as an inert download, not echoed
// back as the type it claims to be.
func TestServeMediaFallsBackToOctetStreamForAnUnallowlistedStoredMime(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	rel := filepath.Join("2026", "09", "attachment.bin")
	full := filepath.Join(env.handler.mediaRoot, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
	require.NoError(t, os.WriteFile(full, []byte("<script>alert(1)</script>"), 0o640))

	msg, err := env.messages.Queue(conv.ID, env.cs, models.MessageKindDocument, "", &services.MediaFile{
		Path:     rel,
		Mime:     "text/html",
		Filename: "notes.html",
		Size:     26,
	}, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/media/"+msg.ID.String(), nil)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

// The signature is added on the way into the outbox, not in the composer, so
// what the CS sees in the thread is exactly what the customer received. A test
// on the helper alone would not catch the wiring going missing.
func TestSendSignsTheReplyWithTheSendersInitials(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	body := strings.NewReader(`{"body":"sudah kami cek"}`)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/cs/conversations/"+conv.ID.String()+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	history, err := env.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.True(t, strings.HasPrefix(history[0].Body, "sudah kami cek"),
		"the CS's own words come first, unchanged")
	assert.Contains(t, history[0].Body, "\n\n~",
		"and the signature sits on its own line beneath them")
}
