# CS Presence on Firebase RTDB Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make an agent leave the CS inbox the moment their socket closes, instead of up to sixty seconds later.

**Architecture:** The browser writes its own presence node to Firebase Realtime Database with `onDisconnect()`, and reads the whole set back with a live listener, so the team panel is instant. Separately, the `api` process polls RTDB every fifteen seconds and mirrors the result into the `cs:online:*` Redis keys that already exist, so `wa` — which runs the round-robin and holds the WhatsApp sessions — is not touched at all.

**Tech Stack:** Go 1.25, `firebase.google.com/go/v4` (already present), Redis 7, React 18, `firebase@12` (already present), Vitest, testify.

**Spec:** `docs/superpowers/specs/2026-09-05-cs-presence-rtdb-design.md`

## Global Constraints

- The `wa` service is never modified and never restarted by this work. `internal/wa/**` and the `wa` block of `docker-compose.yml` must be byte-identical at the end.
- `CSAssignmentService` and its tests are not modified. Their staying green untouched is the evidence `wa` was not disturbed.
- `CsTeamPanel.tsx` and `__tests__/CsTeamPanel.test.tsx` are not modified. The component already takes `online: string[]`.
- An unconfigured Firebase project must leave the app fully working, exactly as it does today: no panics, no failed startup, presence simply inert. `push.NewClient` returning `(nil, nil)` on empty config is the pattern to follow.
- Redis is a projection with exactly one writer. Nothing reads `cs:online:*` to decide what RTDB should contain.
- `NextTurn` stays an `INCR` in Redis and is not touched.
- Every backend commit must leave `gofmt -s -l .` empty and `go vet ./...` clean. Every frontend commit must leave `npm run format:check`, `npm run lint` (0 errors), and `npx tsc --noEmit` clean.
- Prerequisite outside the code: the RTDB instance must exist and its rules published before Task 5 can be verified against a real browser. Tasks 1–4 and 6–7 are verifiable without it.

---

### Task 1: Share one Firebase app between push and presence

`push.NewClient` builds a `firebase.App` privately and returns only a messaging client. Auth (for custom tokens) and Database (for the mirror) need that same app, and building a second one from the same service account would be two clients where one belongs.

**Files:**
- Create: `backend/internal/firebaseapp/app.go`
- Create: `backend/internal/firebaseapp/app_test.go`
- Modify: `backend/internal/push/client.go:27-48`
- Modify: `backend/cmd/api/main.go:120-129`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `firebaseapp.New(ctx context.Context, serviceAccountJSONB64 string) (*firebase.App, error)`, returning `(nil, nil)` for an empty string. `push.NewClient(ctx context.Context, app *firebase.App) (*Client, error)`, returning `(nil, nil)` for a nil app.

- [ ] **Step 1: Write the failing test**

`backend/internal/firebaseapp/app_test.go`:

```go
package firebaseapp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A checkout with no Firebase project must still start. Every caller reads a
// nil app as "Firebase is not configured", so this may not be an error.
func TestNewReturnsNothingWhenUnconfigured(t *testing.T) {
	app, err := New(context.Background(), "")

	require.NoError(t, err)
	assert.Nil(t, app)
}

// A malformed key is a deployment mistake and has to be loud, not silently
// equivalent to having no Firebase at all.
func TestNewRejectsAKeyThatIsNotBase64(t *testing.T) {
	_, err := New(context.Background(), "not base64 at all!!")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode FIREBASE_SERVICE_ACCOUNT_JSON_B64")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/firebaseapp/ -v`
Expected: FAIL — the package does not exist.

- [ ] **Step 3: Write the implementation**

`backend/internal/firebaseapp/app.go`:

```go
// Package firebaseapp builds the one Firebase app the backend uses.
//
// Push, custom tokens and the presence mirror are three clients off a single
// service account; constructing an app per consumer would open three
// connections to the same project and give each its own failure mode.
package firebaseapp

import (
	"context"
	"encoding/base64"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

// New builds the Firebase app from a base64-encoded service account JSON key.
//
// An empty key returns (nil, nil), never an error: a fresh checkout, or a
// deploy made before the Firebase project exists, must still start. Callers
// read a nil app as "Firebase is not configured".
func New(ctx context.Context, serviceAccountJSONB64 string) (*firebase.App, error) {
	if serviceAccountJSONB64 == "" {
		return nil, nil
	}

	raw, err := base64.StdEncoding.DecodeString(serviceAccountJSONB64)
	if err != nil {
		return nil, fmt.Errorf("decode FIREBASE_SERVICE_ACCOUNT_JSON_B64: %w", err)
	}

	app, err := firebase.NewApp(ctx, nil, option.WithAuthCredentialsJSON(option.ServiceAccount, raw))
	if err != nil {
		return nil, fmt.Errorf("initialize firebase app: %w", err)
	}
	return app, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/firebaseapp/ -v`
Expected: PASS, both tests.

- [ ] **Step 5: Move push onto the shared app**

Replace the body of `NewClient` in `backend/internal/push/client.go` (currently lines 27-48) with:

```go
// NewClient builds a Client from the shared Firebase app.
//
// A nil app means Firebase is not configured, and returns (nil, nil) rather
// than an error — the caller treats a nil *Client as "push is not configured"
// (see cmd/api/main.go).
func NewClient(ctx context.Context, app *firebase.App) (*Client, error) {
	if app == nil {
		return nil, nil
	}

	fcm, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase messaging client: %w", err)
	}

	return &Client{fcm: fcm}, nil
}
```

Delete the now-unused `encoding/base64` and `google.golang.org/api/option` imports from that file. Keep `firebase "firebase.google.com/go/v4"`.

- [ ] **Step 6: Update the caller**

In `backend/cmd/api/main.go`, replace lines 120-129 with:

```go
	firebaseApp, err := firebaseapp.New(context.Background(), cfg.FirebaseServiceAccountJSONB64)
	if err != nil {
		log.Warn("Firebase is not available", zap.Error(err))
	}

	pushClient, err := push.NewClient(context.Background(), firebaseApp)
	if err != nil {
		log.Warn("Push notifications are not available", zap.Error(err))
	} else if pushClient != nil {
		pushNotifier.SetSender(pushClient)
		go pushListener.Run(context.Background())
		log.Info("Push notification listener started")
	} else {
		log.Info("FIREBASE_SERVICE_ACCOUNT_JSON_B64 not set — push notifications disabled")
	}
```

Add `"github.com/tikman/olt-provisioning/internal/firebaseapp"` to the imports.

- [ ] **Step 7: Run the affected suites**

Run: `cd backend && go build ./... && go test ./internal/push/... ./internal/firebaseapp/... -v`
Expected: PASS. If a push test constructed a client from a base64 string, change it to build an app with `firebaseapp.New` first; do not change what it asserts.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/firebaseapp backend/internal/push/client.go backend/cmd/api/main.go
git commit -m "refactor(firebase): build the app once and share it

Push held the only firebase.App and built it privately from the service
account. Custom tokens and the presence mirror need the same app, and
three constructions of one project would be three connections with three
failure modes."
```

---

### Task 2: Carry the database URL to both processes

RTDB needs a URL the six existing Firebase values do not include. The browser's copy is inlined at build time, so it travels through the Dockerfile and Compose as a build arg; missing one of those places yields a bundle that silently reads `""`, which `firebase.ts` records as having gone wrong once already.

**Files:**
- Modify: `backend/internal/config/config.go` (near `FirebaseServiceAccountJSONB64`, line ~39 and ~84)
- Modify: `frontend/src/shared/config/firebase.ts:16-23`
- Modify: `frontend/Dockerfile:15-28`
- Modify: `docker-compose.yml` (the `frontend` service's `build.args`, and the `api` service's `environment`)
- Modify: `.env.example`

**Interfaces:**
- Consumes: nothing.
- Produces: `cfg.FirebaseDatabaseURL string` in Go; `firebaseConfig.databaseURL` in the browser.

- [ ] **Step 1: Add the Go config field**

In `backend/internal/config/config.go`, beside `FirebaseServiceAccountJSONB64`:

```go
	// FirebaseDatabaseURL is the Realtime Database the presence mirror reads.
	// Empty means presence is not mirrored, which leaves the round-robin with
	// nobody online — the same state as an unconfigured Firebase project.
	FirebaseDatabaseURL string
```

and in the loader beside the existing Firebase line:

```go
		FirebaseDatabaseURL: viper.GetString("FIREBASE_DATABASE_URL"),
```

- [ ] **Step 2: Add the browser config value**

In `frontend/src/shared/config/firebase.ts`, inside `firebaseConfig`, after `appId`:

```ts
  databaseURL: import.meta.env.VITE_FIREBASE_DATABASE_URL || "",
```

and in `frontend/src/vite-env.d.ts`, inside `ImportMetaEnv`:

```ts
  readonly VITE_FIREBASE_DATABASE_URL: string;
```

- [ ] **Step 3: Add it to the Dockerfile**

In `frontend/Dockerfile`, add to the ARG block (after `ARG VITE_FIREBASE_VAPID_KEY=`):

```dockerfile
ARG VITE_FIREBASE_DATABASE_URL=
```

and add a line to the `ENV` block, keeping the trailing backslashes correct:

```dockerfile
    VITE_FIREBASE_VAPID_KEY=$VITE_FIREBASE_VAPID_KEY \
    VITE_FIREBASE_DATABASE_URL=$VITE_FIREBASE_DATABASE_URL
```

- [ ] **Step 4: Add it to Compose**

In `docker-compose.yml`, under the `frontend` service's `build.args`, after the `VITE_FIREBASE_VAPID_KEY` line:

```yaml
        VITE_FIREBASE_DATABASE_URL: ${VITE_FIREBASE_DATABASE_URL:-}
```

and under the **`api`** service's `environment` (not `wa`):

```yaml
      - FIREBASE_DATABASE_URL=${FIREBASE_DATABASE_URL:-}
```

In `.env.example`, beside the other Firebase entries:

```
FIREBASE_DATABASE_URL=
VITE_FIREBASE_DATABASE_URL=
```

- [ ] **Step 5: Verify the value survives a build**

Run:

```bash
cd frontend && VITE_FIREBASE_DATABASE_URL=https://plan-check.example.com npm run build \
  && grep -c "plan-check.example.com" dist/assets/*.js && rm -rf dist
```

Expected: at least one match. A zero here is the exact failure `firebase.ts` warns about, and means one of Steps 2–4 was missed.

- [ ] **Step 6: Verify the backend still builds and starts unconfigured**

Run: `cd backend && go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/config/config.go frontend/src/shared/config/firebase.ts \
  frontend/src/vite-env.d.ts frontend/Dockerfile docker-compose.yml .env.example
git commit -m "feat(firebase): carry the Realtime Database URL to api and the bundle

Vite inlines the browser copy at build time, so it has to reach the
build through Dockerfile ARG/ENV and Compose build.args, not the
container. FIREBASE_DATABASE_URL goes to api only; wa must not gain
Firebase configuration, because recreating it drops the WhatsApp
sessions."
```

---

### Task 3: Mint Firebase custom tokens

The browser has no Firebase identity — the app authenticates with session cookies against Go. Without one, RTDB rules have no `auth.uid` to rest on and the only writable configuration is one anybody could abuse.

**Files:**
- Create: `backend/internal/api/auth_firebase.go`
- Create: `backend/internal/api/auth_firebase_test.go`
- Modify: `backend/internal/api/router.go` (the `auth` group, near line 120)

**Interfaces:**
- Consumes: `firebaseapp.New` from Task 1.
- Produces: `GET /api/v1/auth/firebase-token` answering `{"data":{"token":"..."}}`; `api.Setup` gains a `*firebase.App` parameter.

- [ ] **Step 1: Write the failing test**

`backend/internal/api/auth_firebase_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

func firebaseTokenRouter(t *testing.T, h *FirebaseTokenHandler, id uuid.UUID, role models.UserRole) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", id)
		c.Set("user_role", role)
		c.Next()
	})
	r.GET("/api/v1/auth/firebase-token", h.Token)
	return r
}

// The whole app is built to run without a Firebase project. An unconfigured
// deployment must say so plainly rather than fail as if something broke.
func TestFirebaseTokenSaysSoWhenFirebaseIsNotConfigured(t *testing.T) {
	h := NewFirebaseTokenHandler(nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/firebase-token", nil)
	rec := httptest.NewRecorder()
	firebaseTokenRouter(t, h, uuid.New(), models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "FIREBASE_NOT_CONFIGURED")
}

// The id comes from the session and nowhere else. A token minted for an id in
// the request would let any agent write another agent's presence node.
func TestFirebaseTokenRefusesWithoutASession(t *testing.T) {
	h := NewFirebaseTokenHandler(nil, zap.NewNop())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/auth/firebase-token", h.Token)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/firebase-token", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/api/ -run TestFirebaseToken -v`
Expected: FAIL — `NewFirebaseTokenHandler` undefined.

- [ ] **Step 3: Write the handler**

`backend/internal/api/auth_firebase.go`:

```go
package api

import (
	"net/http"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FirebaseTokenHandler mints the short-lived identity a browser needs before
// it may write its own presence node.
type FirebaseTokenHandler struct {
	app    *firebase.App
	logger *zap.Logger
}

// NewFirebaseTokenHandler constructs a FirebaseTokenHandler. A nil app means
// Firebase is not configured.
func NewFirebaseTokenHandler(app *firebase.App, logger *zap.Logger) *FirebaseTokenHandler {
	return &FirebaseTokenHandler{app: app, logger: logger}
}

// Token returns a Firebase custom token for the caller's own user id.
//
// The id is read from the session and never from the request: a token minted
// for an id the caller supplied would let any agent claim to be another, which
// is precisely what the RTDB rules use auth.uid to prevent.
func (h *FirebaseTokenHandler) Token(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	userID, ok := raw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}

	if h.app == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error: "Firebase is not configured",
			Code:  "FIREBASE_NOT_CONFIGURED",
		})
		return
	}

	client, err := h.app.Auth(c.Request.Context())
	if err != nil {
		h.logger.Error("firebase auth client", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to mint token", Code: "TOKEN_FAILED",
		})
		return
	}

	token, err := client.CustomToken(c.Request.Context(), userID.String())
	if err != nil {
		h.logger.Error("mint firebase custom token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to mint token", Code: "TOKEN_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": token}})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/api/ -run TestFirebaseToken -v`
Expected: PASS, both tests.

- [ ] **Step 5: Register the route**

`api.Setup` must accept the app. Change its signature to take `firebaseApp *firebase.App` after `wgService`, construct `firebaseTokenHandler := NewFirebaseTokenHandler(firebaseApp, logger)` beside the other handlers, and register inside the authenticated part of the `auth` group:

```go
			auth.GET("/firebase-token",
				middleware.AuthMiddleware(authStore, logger),
				middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician),
				firebaseTokenHandler.Token)
```

The `auth` group has no authenticated sub-group: `/me` (router.go:121) carries its own middleware inline, and the block above follows that shape exactly.

Update `cmd/api/main.go` to pass `firebaseApp` (from Task 1) into `api.Setup`. Update every other `api.Setup` caller the compiler names — test helpers included — passing `nil`.

- [ ] **Step 6: Run the API suite**

Run: `cd backend && go build ./... && go test ./internal/api/`
Expected: PASS. `nil` in the test helpers gives `503`, which no existing test asserts against.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/api/auth_firebase.go backend/internal/api/auth_firebase_test.go \
  backend/internal/api/router.go backend/cmd/api/main.go
git commit -m "feat(auth): mint Firebase custom tokens for the session's own user

RTDB rules need an auth.uid to restrict a write to the agent's own
presence node. The app authenticates with session cookies and has no
Firebase identity, so the backend mints one — for the id in the session,
never one the caller supplied."
```

---

### Task 4: Mirror RTDB presence into Redis

**Files:**
- Create: `backend/internal/services/cs_presence_mirror.go`
- Create: `backend/internal/services/cs_presence_mirror_test.go`
- Create: `backend/internal/firebaseapp/rtdb_presence.go`
- Modify: `backend/internal/services/cs_presence.go` (the `Presence` interface, `RedisPresence`, `FakePresence`)
- Modify: `backend/cmd/api/main.go`

**Interfaces:**
- Consumes: `firebaseapp.New` (Task 1), `cfg.FirebaseDatabaseURL` (Task 2).
- Produces: `services.PresenceSnapshot` interface with `Present(ctx context.Context) ([]uuid.UUID, error)`; `services.NewPresenceMirror(snapshot PresenceSnapshot, presence Presence, logger *zap.Logger) *PresenceMirror` with methods `Sync(ctx context.Context) error` and `Run(ctx context.Context)`; `firebaseapp.NewRTDBPresence(ctx, app, databaseURL) (*RTDBPresence, error)`.

- [ ] **Step 1: Add MarkOffline to the presence seam**

In `backend/internal/services/cs_presence.go`, add to the `Presence` interface:

```go
	MarkOffline(ctx context.Context, userID uuid.UUID) error
```

to `RedisPresence`:

```go
// MarkOffline drops an agent from the online set immediately, rather than
// waiting out the TTL. The TTL is the outage path; a departure the mirror can
// see must not have to wait sixty seconds for it.
func (p *RedisPresence) MarkOffline(ctx context.Context, userID uuid.UUID) error {
	return p.client.Del(ctx, presenceKeyPrefix+userID.String()).Err()
}
```

and to `FakePresence`:

```go
// MarkOffline removes a user from the online set.
func (p *FakePresence) MarkOffline(_ context.Context, userID uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	kept := p.online[:0]
	for _, id := range p.online {
		if id != userID {
			kept = append(kept, id)
		}
	}
	p.online = kept
	return nil
}
```

- [ ] **Step 2: Write the failing test**

`backend/internal/services/cs_presence_mirror_test.go`:

```go
package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeSnapshot struct {
	ids []uuid.UUID
	err error
}

func (f *fakeSnapshot) Present(context.Context) ([]uuid.UUID, error) {
	return f.ids, f.err
}

func TestMirrorWritesEveryAgentTheSnapshotHolds(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	presence := NewFakePresence()
	mirror := NewPresenceMirror(&fakeSnapshot{ids: []uuid.UUID{first, second}}, presence, zap.NewNop())

	require.NoError(t, mirror.Sync(context.Background()))

	online, err := presence.Online(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{first, second}, online)
}

// The point of the whole migration. Letting a departed agent's key expire
// instead would leave assignment as slow as the sixty-second TTL it replaces.
func TestMirrorDeletesAnAgentTheSnapshotNoLongerHolds(t *testing.T) {
	staying, leaving := uuid.New(), uuid.New()
	presence := NewFakePresence()
	snapshot := &fakeSnapshot{ids: []uuid.UUID{staying, leaving}}
	mirror := NewPresenceMirror(snapshot, presence, zap.NewNop())
	require.NoError(t, mirror.Sync(context.Background()))

	snapshot.ids = []uuid.UUID{staying}
	require.NoError(t, mirror.Sync(context.Background()))

	online, err := presence.Online(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{staying}, online)
}

// One failed read is a blip, not an evacuation. Emptying the rotation on it
// would stop assignment for everybody over a single dropped request; the TTL
// is what handles a real outage, by expiring keys nobody refreshes.
func TestMirrorLeavesTheSetAloneWhenASnapshotFails(t *testing.T) {
	agent := uuid.New()
	presence := NewFakePresence()
	snapshot := &fakeSnapshot{ids: []uuid.UUID{agent}}
	mirror := NewPresenceMirror(snapshot, presence, zap.NewNop())
	require.NoError(t, mirror.Sync(context.Background()))

	snapshot.err = errors.New("rtdb unreachable")
	require.Error(t, mirror.Sync(context.Background()))

	online, err := presence.Online(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{agent}, online)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestMirror -v`
Expected: FAIL — `NewPresenceMirror` undefined.

- [ ] **Step 4: Write the mirror**

`backend/internal/services/cs_presence_mirror.go`:

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mirrorInterval is how stale the round-robin's view of presence can be.
//
// It is a poll rather than a subscription because the Go Admin SDK has no
// listener for the Realtime Database: db.Ref offers Get, GetShallow, Set,
// Delete and Transaction over REST and nothing that streams. The browser has a
// real listener, so the team panel updates instantly and this does not.
const mirrorInterval = 15 * time.Second

// PresenceSnapshot reads who the Realtime Database currently holds.
type PresenceSnapshot interface {
	Present(ctx context.Context) ([]uuid.UUID, error)
}

// PresenceMirror projects the Realtime Database's presence into the Redis keys
// the round-robin already reads, so the wa process — which runs assignment and
// holds the WhatsApp sessions — needs no Firebase configuration and no restart.
type PresenceMirror struct {
	snapshot PresenceSnapshot
	presence Presence
	logger   *zap.Logger
	// mirrored is the previous pass's set, so a departure can be deleted
	// rather than waited out.
	mirrored map[uuid.UUID]struct{}
}

// NewPresenceMirror constructs a PresenceMirror.
func NewPresenceMirror(snapshot PresenceSnapshot, presence Presence, logger *zap.Logger) *PresenceMirror {
	return &PresenceMirror{
		snapshot: snapshot,
		presence: presence,
		logger:   logger,
		mirrored: make(map[uuid.UUID]struct{}),
	}
}

// Sync brings Redis into line with one reading of the Realtime Database.
//
// A failed reading changes nothing. One dropped request is a blip, and
// emptying the rotation over it would stop assignment for the whole team; a
// real outage is covered by the keys' own TTL expiring while nothing refreshes
// them.
func (m *PresenceMirror) Sync(ctx context.Context) error {
	present, err := m.snapshot.Present(ctx)
	if err != nil {
		return fmt.Errorf("read presence snapshot: %w", err)
	}

	now := make(map[uuid.UUID]struct{}, len(present))
	for _, id := range present {
		now[id] = struct{}{}
		if err := m.presence.MarkOnline(ctx, id); err != nil {
			return fmt.Errorf("mirror online %s: %w", id, err)
		}
	}

	for id := range m.mirrored {
		if _, still := now[id]; still {
			continue
		}
		if err := m.presence.MarkOffline(ctx, id); err != nil {
			return fmt.Errorf("mirror offline %s: %w", id, err)
		}
	}

	m.mirrored = now
	return nil
}

// Run mirrors until the context is cancelled.
func (m *PresenceMirror) Run(ctx context.Context) {
	ticker := time.NewTicker(mirrorInterval)
	defer ticker.Stop()

	for {
		if err := m.Sync(ctx); err != nil {
			m.logger.Warn("mirror CS presence", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd backend && go test ./internal/services/ -run TestMirror -v`
Expected: PASS, all three.

- [ ] **Step 6: Write the RTDB snapshot source**

`backend/internal/firebaseapp/rtdb_presence.go`:

```go
package firebaseapp

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"github.com/google/uuid"
)

// presencePath is the node the browser writes its own presence to. It must
// match the security rules and the browser's path exactly.
const presencePath = "cs-presence"

// RTDBPresence reads the presence set out of the Realtime Database.
type RTDBPresence struct {
	ref *db.Ref
}

// NewRTDBPresence constructs an RTDBPresence. A nil app or an empty URL
// returns (nil, nil): presence is simply not mirrored.
func NewRTDBPresence(ctx context.Context, app *firebase.App, databaseURL string) (*RTDBPresence, error) {
	if app == nil || databaseURL == "" {
		return nil, nil
	}
	client, err := app.DatabaseWithURL(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase database client: %w", err)
	}
	return &RTDBPresence{ref: client.NewRef(presencePath)}, nil
}

// Present lists the agents holding the inbox open.
//
// GetShallow returns the keys without their values, which is all this needs
// and keeps the response one entry per agent however much the nodes grow.
func (p *RTDBPresence) Present(ctx context.Context) ([]uuid.UUID, error) {
	var shallow map[string]bool
	if err := p.ref.GetShallow(ctx, &shallow); err != nil {
		return nil, fmt.Errorf("read %s: %w", presencePath, err)
	}

	ids := make([]uuid.UUID, 0, len(shallow))
	for key := range shallow {
		id, err := uuid.Parse(key)
		if err != nil {
			// A node whose key is not a user id was not written by this app.
			// Skipping it is right; failing the whole pass over it would let
			// one stray key stop the rotation.
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
```

- [ ] **Step 7: Start the mirror**

In `backend/cmd/api/main.go`, after the push wiring from Task 1:

```go
	rtdbPresence, err := firebaseapp.NewRTDBPresence(context.Background(), firebaseApp, cfg.FirebaseDatabaseURL)
	if err != nil {
		log.Warn("CS presence mirror is not available", zap.Error(err))
	} else if rtdbPresence != nil {
		mirror := services.NewPresenceMirror(rtdbPresence, sessionPresence, log)
		go mirror.Run(context.Background())
		log.Info("CS presence mirror started")
	} else {
		log.Info("FIREBASE_DATABASE_URL not set — CS presence is not mirrored")
	}
```

`Setup` today returns `(*gin.Engine, *services.PushNotifierService, *PushEventListener)` and builds the presence at `router.go:88` as `csPresence := services.NewRedisPresence(csRedisClient)`. Add it as a fourth return value and take it here as `csPresence`. Do **not** construct a second `RedisPresence` in `main.go` — it would be a second object over one key space, and the Redis client it needs is the dedicated CS connection the router owns.

- [ ] **Step 8: Run the full backend suite**

Run: `cd backend && gofmt -s -l . && go vet ./... && go test ./... -race`
Expected: `gofmt` prints nothing, vet clean, all packages pass. `internal/services` assignment tests must be untouched and green.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/services/cs_presence.go backend/internal/services/cs_presence_mirror.go \
  backend/internal/services/cs_presence_mirror_test.go backend/internal/firebaseapp/rtdb_presence.go \
  backend/internal/api/router.go backend/cmd/api/main.go
git commit -m "feat(cs): mirror RTDB presence into the Redis keys wa already reads

Assignment runs in wa, which has no Firebase credentials; giving it some
means recreating the container that holds the WhatsApp sessions. So api
polls the Realtime Database and projects it into cs:online:*, and wa is
not touched.

A departed agent's key is deleted rather than left to expire — waiting
out the TTL is the lag this migration exists to remove. A failed poll
changes nothing, because one dropped request is not an evacuation."
```

---

### Task 5: Write presence from the browser

**Files:**
- Create: `frontend/src/infrastructure/firebase/presence.ts`
- Create: `frontend/src/infrastructure/firebase/presence.test.ts`
- Modify: `frontend/src/infrastructure/repositories/CsRepository.ts`
- Modify: `frontend/src/domain/repositories/ICsRepository.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`

**Interfaces:**
- Consumes: `GET /api/v1/auth/firebase-token` (Task 3), `firebaseConfig.databaseURL` (Task 2).
- Produces: `claimPresence(userId: string): Promise<() => void>` — registers the disconnect handler, writes the node, and returns a function that removes it.

- [ ] **Step 1: Write the failing test**

`frontend/src/infrastructure/firebase/presence.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { writePresence } from "./presence";

describe("writePresence", () => {
  // A socket that dies between the two calls must leave nothing behind. If the
  // value is written first, that window leaves a ghost online until the
  // mirror's next pass.
  it("arms the disconnect handler before writing the value", async () => {
    const order: string[] = [];
    const ref = {
      onDisconnect: () => ({
        remove: async () => {
          order.push("armed");
        },
      }),
      set: async () => {
        order.push("written");
      },
      remove: async () => {
        order.push("removed");
      },
    };

    const release = await writePresence(ref);

    expect(order).toEqual(["armed", "written"]);
    await release();
    expect(order).toEqual(["armed", "written", "removed"]);
  });

  // Leaving the page is a normal exit and must clear the node itself, rather
  // than relying on the socket closing a moment later.
  it("removes the node when released", async () => {
    const remove = vi.fn(async () => {});
    const ref = {
      onDisconnect: () => ({ remove: async () => {} }),
      set: async () => {},
      remove,
    };

    const release = await writePresence(ref);
    await release();

    expect(remove).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npm test -- --run src/infrastructure/firebase/presence.test.ts`
Expected: FAIL — cannot resolve `./presence`.

- [ ] **Step 3: Write the module**

`frontend/src/infrastructure/firebase/presence.ts`:

```ts
import { initializeApp, getApps, type FirebaseApp } from "firebase/app";
import { getAuth, signInWithCustomToken } from "firebase/auth";
import {
  getDatabase,
  ref,
  serverTimestamp,
  onDisconnect,
  onValue,
  set,
  remove,
} from "firebase/database";
import { firebaseConfig } from "@/shared/config/firebase";
import { CsRepository } from "@/infrastructure/repositories";

const csRepository = new CsRepository();

/** The path the security rules and the Go mirror both name. Changing it here
 * alone silently breaks both. */
const PRESENCE_PATH = "cs-presence";

export const isPresenceConfigured =
  !!firebaseConfig.apiKey && !!firebaseConfig.databaseURL;

/** The narrow shape writePresence needs, so its ordering can be tested
 * without the Firebase SDK. */
export interface PresenceRef {
  onDisconnect: () => { remove: () => Promise<unknown> };
  set: (value: unknown) => Promise<unknown>;
  remove: () => Promise<unknown>;
}

/**
 * Claims a presence node and returns the function that gives it up.
 *
 * The disconnect handler is armed BEFORE the value is written. Written first,
 * a socket dying in the gap would leave a node nothing ever removes, and the
 * agent would show as present until the mirror's next pass — the exact failure
 * this design exists to remove.
 */
export async function writePresence(node: PresenceRef): Promise<() => Promise<void>> {
  await node.onDisconnect().remove();
  await node.set({ at: serverTimestamp() });
  return async () => {
    await node.remove();
  };
}

function app(): FirebaseApp {
  return getApps()[0] ?? initializeApp(firebaseConfig);
}

/** Signs this browser in as its own user and claims presence for it. Resolves
 * to a no-op when Firebase is not configured, so callers need no branch. */
export async function claimPresence(): Promise<() => Promise<void>> {
  if (!isPresenceConfigured) return async () => {};

  const token = await csRepository.getFirebaseToken();
  const auth = getAuth(app());
  const credential = await signInWithCustomToken(auth, token);

  const node = ref(getDatabase(app()), `${PRESENCE_PATH}/${credential.user.uid}`);
  return writePresence({
    onDisconnect: () => onDisconnect(node),
    set: (value) => set(node, value),
    remove: () => remove(node),
  });
}

/** Subscribes to the whole presence set. Returns the unsubscribe function. */
export function watchPresence(onChange: (ids: string[]) => void): () => void {
  if (!isPresenceConfigured) {
    onChange([]);
    return () => {};
  }
  const node = ref(getDatabase(app()), PRESENCE_PATH);
  return onValue(node, (snapshot) => {
    const value = (snapshot.val() ?? {}) as Record<string, unknown>;
    onChange(Object.keys(value));
  });
}
```

- [ ] **Step 4: Add the token fetch to the repository**

In `frontend/src/infrastructure/http/endpoints.ts`, beside the auth entries:

```ts
  AUTH_FIREBASE_TOKEN: "/api/v1/auth/firebase-token",
```

In `frontend/src/domain/repositories/ICsRepository.ts`, replace the `getOnlineAgents` line added in `c02947c` with:

```ts
  /** A Firebase custom token for the signed-in user. */
  getFirebaseToken(): Promise<string>;
```

In `frontend/src/infrastructure/repositories/CsRepository.ts`, replace `getOnlineAgents` with:

```ts
  async getFirebaseToken(): Promise<string> {
    const response = await apiClient.get(API_ENDPOINTS.AUTH_FIREBASE_TOKEN);
    return response.data.data.token;
  }
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && npm test -- --run src/infrastructure/firebase/presence.test.ts && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/infrastructure/firebase/presence.ts \
  frontend/src/infrastructure/firebase/presence.test.ts \
  frontend/src/infrastructure/repositories/CsRepository.ts \
  frontend/src/domain/repositories/ICsRepository.ts \
  frontend/src/infrastructure/http/endpoints.ts
git commit -m "feat(cs): claim presence in RTDB with a disconnect handler

The handler is armed before the value is written. Written first, a socket
dying in the gap leaves a node nothing removes, and the agent shows as
present until the mirror's next pass — the failure this design exists to
remove."
```

---

### Task 6: Point the panel at RTDB and delete the polled path

**Files:**
- Modify: `frontend/src/application/hooks/useCsInbox.ts` (the `useOnlineAgents` added in `c02947c`)
- Modify: `frontend/src/presentation/pages/CsInboxPage.tsx`
- Modify: `frontend/src/shared/config/limits.ts` (remove `CS_PRESENCE_POLL_MS`)
- Delete: `backend/internal/api/cs_handler_online.go`, `backend/internal/api/cs_handler_online_test.go`
- Modify: `backend/internal/api/router.go`, `backend/internal/api/cs_handler_test.go` (remove the `/online` route from both)
- Modify: `frontend/src/infrastructure/http/endpoints.ts` (remove `CS_ONLINE`)

**Interfaces:**
- Consumes: `watchPresence` and `claimPresence` from Task 5.
- Produces: `useOnlineAgents(): { data: string[] }` — same shape the page already consumes, so `CsInboxPage`'s use of it does not change.

- [ ] **Step 1: Write the failing test**

`frontend/src/application/hooks/useOnlineAgents.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const unsubscribe = vi.fn();
let emit: (ids: string[]) => void = () => {};

vi.mock("@/infrastructure/firebase/presence", () => ({
  watchPresence: (onChange: (ids: string[]) => void) => {
    emit = onChange;
    return unsubscribe;
  },
  claimPresence: async () => async () => {},
}));

import { useOnlineAgents } from "./useOnlineAgents";

describe("useOnlineAgents", () => {
  beforeEach(() => unsubscribe.mockClear());

  it("hands on whatever the subscription reports", () => {
    const { result } = renderHook(() => useOnlineAgents());

    act(() => emit(["u-rina", "u-budi"]));

    expect(result.current.data).toEqual(["u-rina", "u-budi"]);
  });

  // A subscription left open outlives the page and keeps a socket per visit.
  it("unsubscribes when the page goes away", () => {
    const { unmount } = renderHook(() => useOnlineAgents());

    unmount();

    expect(unsubscribe).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npm test -- --run src/application/hooks/useOnlineAgents.test.ts`
Expected: FAIL — cannot resolve `./useOnlineAgents`.

- [ ] **Step 3: Write the hook**

Create `frontend/src/application/hooks/useOnlineAgents.ts`:

```ts
import { useEffect, useState } from "react";
import { watchPresence } from "@/infrastructure/firebase/presence";

/**
 * The agents holding the inbox open, straight off the Realtime Database.
 *
 * Not React Query: there is nothing to fetch and nothing to invalidate. The
 * subscription pushes, and a query cache in front of it would only add a copy
 * that can be stale.
 */
export function useOnlineAgents(): { data: string[] } {
  const [ids, setIds] = useState<string[]>([]);

  useEffect(() => watchPresence(setIds), []);

  return { data: ids };
}
```

Remove `useOnlineAgents` and the `CS_PRESENCE_POLL_MS` import from `useCsInbox.ts`, and export the new module from `frontend/src/application/hooks/index.ts`:

```ts
export * from "./useOnlineAgents";
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npm test -- --run src/application/hooks/useOnlineAgents.test.ts`
Expected: PASS, both.

- [ ] **Step 5: Claim presence from the page**

In `frontend/src/presentation/pages/CsInboxPage.tsx`, beside the existing hook calls:

```ts
  // Only this page claims presence — the same rule the SSE stream's
  // ?presence=1 followed: someone reading the OLT map is not at the inbox.
  useEffect(() => {
    let release: (() => Promise<void>) | undefined;
    let cancelled = false;

    claimPresence()
      .then((r) => {
        if (cancelled) {
          void r();
          return;
        }
        release = r;
      })
      .catch((error) => console.warn("Could not claim presence", error));

    return () => {
      cancelled = true;
      void release?.();
    };
  }, []);
```

Import `claimPresence` from `@/infrastructure/firebase/presence`. The `cancelled` flag matters for the same reason it does in `AppLayout`: StrictMode mounts, unmounts and remounts synchronously in development, and a release that arrives after cleanup would otherwise never run.

- [ ] **Step 6: Say when the panel has stopped hearing**

The spec left this decision here deliberately. `onValue` keeps its last snapshot
when the socket drops, so a browser that loses RTDB shows the team frozen as it
last saw them and does not know it is stale. Silence that looks like data is the
worst of the options, so the page says so.

RTDB publishes its own connection state at `.info/connected`. Add to
`frontend/src/infrastructure/firebase/presence.ts`:

```ts
/** Subscribes to RTDB's own view of whether this browser is connected.
 * Reports false when Firebase is not configured, which is the truth. */
export function watchConnection(onChange: (connected: boolean) => void): () => void {
  if (!isPresenceConfigured) {
    onChange(false);
    return () => {};
  }
  const node = ref(getDatabase(app()), ".info/connected");
  return onValue(node, (snapshot) => onChange(snapshot.val() === true));
}
```

Extend `useOnlineAgents` to return it:

```ts
export function useOnlineAgents(): { data: string[]; connected: boolean } {
  const [ids, setIds] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);

  useEffect(() => watchPresence(setIds), []);
  useEffect(() => watchConnection(setConnected), []);

  return { data: ids, connected };
}
```

and update the two assertions in `useOnlineAgents.test.ts` to mock
`watchConnection` alongside `watchPresence`, plus one new test:

```ts
  // A frozen list that looks live is worse than one that admits it is stale.
  it("reports the connection separately from the set", () => {
    const { result } = renderHook(() => useOnlineAgents());

    act(() => emitConnected(false));

    expect(result.current.connected).toBe(false);
  });
```

In `CsInboxPage.tsx`, render the note **above** `CsTeamPanel`, inside the
wrapper div — `CsTeamPanel.tsx` is unchanged by the constraints at the top of
this plan:

```tsx
              {!onlineQuery.connected && (
                <Text
                  style={{
                    display: "block",
                    padding: "8px 14px 0",
                    color: colors.textMuted,
                    fontSize: 11,
                  }}
                >
                  Terputus — daftar ini mungkin sudah tidak akurat
                </Text>
              )}
```

- [ ] **Step 7: Delete the polled path**

- `rm backend/internal/api/cs_handler_online.go backend/internal/api/cs_handler_online_test.go`
- Remove `cs.GET("/online", csHandler.Online)` from `backend/internal/api/router.go`
- Remove `cs.GET("/online", e.handler.Online)` from `backend/internal/api/cs_handler_test.go`
- Remove `CS_ONLINE` from `frontend/src/infrastructure/http/endpoints.ts`
- Remove `CS_PRESENCE_POLL_MS` from `frontend/src/shared/config/limits.ts`
- In `frontend/src/presentation/pages/__tests__/CsInboxPageView.test.tsx`, the `useOnlineAgents: () => stub` line in the hooks mock stays — the hook still exists, only its implementation changed.

- [ ] **Step 8: Run both suites**

Run:

```bash
cd backend && go build ./... && go vet ./... && go test ./... \
  && cd ../frontend && npm test -- --run && npx tsc --noEmit && npm run lint && npm run format:check
```

Expected: all green. `CsTeamPanel.test.tsx` must pass **unmodified** — that is the evidence the data source swap did not change the component.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(cs): read presence from RTDB and delete the polled endpoint

The panel now updates the instant a node appears or vanishes rather than
within twenty seconds. GET /api/v1/cs/online, its repository method and
its poll interval have no caller left, so they are removed rather than
kept beside their replacement."
```

---

### Task 7: Retire the SSE presence claim

Two things writing presence is one too many. The stream's `MarkOnline` would keep an agent in Redis for sixty seconds after RTDB has dropped them, which is exactly the lag this work removes.

**Files:**
- Modify: `backend/internal/api/cs_handler_stream.go:24-43`
- Modify: `backend/internal/api/cs_handler_conversations.go` (the `presence` field on `CSHandler` and `NewCSHandler`)
- Modify: `backend/internal/api/cs_handler_stream_test.go`
- Modify: `backend/internal/api/router.go`, `backend/internal/api/cs_handler_test.go`
- Modify: `frontend/src/application/hooks/useCsStream.ts` (drop `?presence=1`)

- [ ] **Step 1: Delete the tests that assert the old behaviour**

In `backend/internal/api/cs_handler_stream_test.go`, remove exactly two tests — `TestStreamMarksItsAgentOnlineWhenItClaimsPresence` (line 41) and `TestStreamWithoutPresenceLeavesTheAgentOffline` (line 59) — and the `onlineAgents` helper (line 31), which nothing else calls.

These are not weakened, they are obsolete: the behaviour they describe is being removed on purpose, and a test that pins removed behaviour would block the removal.

- [ ] **Step 2: Run to verify they are gone**

Run: `cd backend && go test ./internal/api/ -run TestStream -v`
Expected: the remaining stream tests pass; the presence ones no longer appear.

- [ ] **Step 3: Remove the presence claim from the stream**

In `cs_handler_stream.go`, delete the `claimsPresence` parse, the `MarkOnline` call at connect, the conditional refresh inside the heartbeat, and the now-unused `strconv` import. The heartbeat itself stays — it is what keeps the SSE connection alive.

Remove the `presence` field from `CSHandler` and its parameter from `NewCSHandler`, then update `router.go` and `cs_handler_test.go` where the handler is constructed. `services.Presence` itself stays: the mirror and the assignment service both use it.

- [ ] **Step 4: Drop the query parameter from the browser**

In `frontend/src/application/hooks/useCsStream.ts`, remove `presence=1` from the stream URL and the argument that decides it. Follow the compiler and the failing tests up to `AppLayout.tsx`, which passes `watchingInbox`; that argument disappears with it.

- [ ] **Step 5: Run both suites**

Run:

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./... -race \
  && cd ../frontend && npm test -- --run && npx tsc --noEmit && npm run lint && npm run format:check && npm run build
```

Expected: all green, `gofmt` silent.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(cs): stop claiming presence from the SSE stream

Two writers of presence is one too many, and this was the slower one: it
kept an agent in Redis for sixty seconds after RTDB had already dropped
them, which is the lag the migration removes. The heartbeat stays — it
keeps the connection alive, which was always its other job."
```

---

## Verification after all tasks

The RTDB instance must exist and its rules be published before this can be checked.

1. Open the CS Inbox in two browsers signed in as different agents. Each sees both rows lit.
2. Close one browser. The other's panel drops that row **immediately**, not after a minute.
3. On the VPS, within fifteen seconds of that:
   `sudo docker exec tikman-redis redis-cli -a "$REDIS_PASSWORD" --scan --pattern 'cs:online:*'`
   lists only the remaining agent.
4. Stop RTDB reachability (block egress, or clear `FIREBASE_DATABASE_URL` and restart `api`). Within sixty seconds the same command lists nothing, and a new inbound WhatsApp message leaves its conversation unassigned rather than being handed to anybody.
5. `sudo docker ps` shows `tikman-wa` with an uptime predating the deploy. If it restarted, the constraint at the top of this plan was broken.
