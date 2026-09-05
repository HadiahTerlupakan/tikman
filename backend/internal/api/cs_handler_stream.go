package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/wa"
)

// heartbeatInterval keeps an idle stream from being taken for a dead one by the
// browser or by whatever ends up proxying it. Presence no longer rides on this
// tick, so a missed beat costs a reconnect and nothing else.
const heartbeatInterval = 15 * time.Second

// Stream keeps one CS browser up to date. It carries no truth of its own: every
// event is a nudge to refetch, which is why a dropped connection costs nothing
// but a moment of staleness.
//
// Presence is claimed by the browser's own RTDB connection now, not by holding
// this stream open, so Stream no longer marks anybody online.
func (h *CSHandler) Stream(c *gin.Context) {
	if _, ok := middleware.GetUserID(c); !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}

	ctx := c.Request.Context()

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
			c.SSEvent("ping", "")
			c.Writer.Flush()
		}
	}
}
