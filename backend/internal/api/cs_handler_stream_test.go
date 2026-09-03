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

// openStream runs one SSE request until the returned cancel is called, and
// closes the returned channel once the handler has actually returned.
func openStream(t *testing.T, env *csHandlerEnv, agent uuid.UUID, target string) (context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		env.asUser(agent, models.UserRoleCS).ServeHTTP(rec, req)
		close(done)
	}()
	return cancel, done
}

func onlineAgents(t *testing.T, env *csHandlerEnv) []uuid.UUID {
	t.Helper()
	online, err := env.presence.Online(context.Background())
	require.NoError(t, err)
	return online
}

// The heartbeat is what keeps a CS in the rotation: presence expires after a
// minute, so a stream that forgets to refresh it quietly drops its own agent
// out of round-robin mid-shift.
func TestStreamMarksItsAgentOnlineWhenItClaimsPresence(t *testing.T) {
	env := setupCSHandler(t)
	agent := uuid.New()

	cancel, done := openStream(t, env, agent, "/api/v1/cs/stream?presence=1")

	require.Eventually(t, func() bool {
		online := onlineAgents(t, env)
		return len(online) == 1 && online[0] == agent
	}, time.Second, 10*time.Millisecond)

	cancel()
	<-done
}

// AppLayout holds this connection open on every page, for a technician looking
// at the OLT map as much as for a CS in the inbox. Marking those online would
// hand round-robin threads to somebody who is not reading the inbox at all.
func TestStreamWithoutPresenceLeavesTheAgentOffline(t *testing.T) {
	env := setupCSHandler(t)
	agent := uuid.New()

	cancel, done := openStream(t, env, agent, "/api/v1/cs/stream")

	require.Never(t, func() bool {
		return len(onlineAgents(t, env)) > 0
	}, 250*time.Millisecond, 10*time.Millisecond)

	cancel()
	<-done
}
