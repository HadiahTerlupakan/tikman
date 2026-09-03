package api

import (
	"net/http"
	"strconv"
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
// The connection is held from the app shell on every page, so holding it is no
// longer evidence that anybody is looking at the inbox. Only a client that asks
// for it with ?presence=1 is marked online, and only the CS Inbox route asks —
// otherwise round-robin would hand threads to a technician reading the OLT map.
func (h *CSHandler) Stream(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}

	// Default off: a client that says nothing is a page other than the inbox,
	// and claiming presence for it is the failure mode this guards against.
	claimsPresence, _ := strconv.ParseBool(c.Query("presence"))

	ctx := c.Request.Context()
	if claimsPresence {
		if err := h.presence.MarkOnline(ctx, userID); err != nil {
			h.logger.Warn("mark CS online", zap.Error(err))
		}
	}

	sub := h.redis.Subscribe(ctx, wa.EventsChannel)
	defer func() { _ = sub.Close() }()
	incoming := sub.Channel()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// A proxy that buffers this connection makes the whole feature silently
	// dead: no error, no log, just an inbox that stops updating. There is no
	// proxy in front of the API today, but there will be one the day someone
	// puts nginx or a CDN there, and by then the symptom is very hard to trace.
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")

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
			// The ping itself keeps every connection alive, presence or not;
			// only the presence refresh is conditional.
			if claimsPresence {
				if err := h.presence.MarkOnline(ctx, userID); err != nil {
					h.logger.Warn("refresh CS presence", zap.Error(err))
				}
			}
			c.SSEvent("ping", "")
			c.Writer.Flush()
		}
	}
}
