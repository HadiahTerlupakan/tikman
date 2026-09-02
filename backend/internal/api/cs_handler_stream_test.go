package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The heartbeat is what keeps a CS in the rotation: presence expires after a
// minute, so a stream that forgets to refresh it quietly drops its own agent
// out of round-robin mid-shift.
func TestStreamMarksItsAgentOnline(t *testing.T) {
	env := setupCSHandler(t)
	agent := uuid.New()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		env.asUser(agent, models.UserRoleCS).ServeHTTP(rec, req)
		close(done)
	}()

	require.Eventually(t, func() bool {
		online, err := env.presence.Online(context.Background())
		return err == nil && len(online) == 1 && online[0] == agent
	}, time.Second, 10*time.Millisecond)

	cancel()
	<-done
}
