package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// heartbeatInterval is well inside the sixty-second presence TTL, so one slow
// moment on the network does not drop a CS out of the rotation.
const heartbeatInterval = 15 * time.Second

// Stream keeps one CS browser up to date. It carries no truth of its own: every
// event is a nudge to refetch, which is why a dropped connection costs nothing
// but a moment of staleness.
//
// Holding the connection open is also what marks this agent online, so the
// rotation only ever hands work to somebody with the inbox actually open.
func (h *CSHandler) Stream(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}

	ctx := c.Request.Context()
	if err := h.presence.MarkOnline(ctx, userID); err != nil {
		h.logger.Warn("mark CS online", zap.Error(err))
	}

	sub := h.redis.Subscribe(ctx, wa.EventsChannel)
	defer func() { _ = sub.Close() }()
	incoming := sub.Channel()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// c.Stream relies on http.ResponseWriter.CloseNotify, which a real server
	// connection provides but httptest.ResponseRecorder does not; watching
	// ctx.Done() ourselves covers client disconnect just as well and is the
	// same signal AuthMiddleware's session refresh and everything else here
	// already trusts.
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-incoming:
			if !open {
				return
			}
			c.SSEvent("cs", msg.Payload)
			c.Writer.Flush()
		case <-ticker.C:
			if err := h.presence.MarkOnline(ctx, userID); err != nil {
				h.logger.Warn("refresh CS presence", zap.Error(err))
			}
			c.SSEvent("ping", "")
			c.Writer.Flush()
		}
	}
}
