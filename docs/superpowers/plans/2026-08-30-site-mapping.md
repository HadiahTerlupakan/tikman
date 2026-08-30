# Site Mapping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show every OLT site as a pin on a map, and capture the coordinates that make those pins possible while creating or editing a site.

**Architecture:** A generic encrypted `app_settings` table holds API credentials, but access is governed by a registry in code where each setting declares who may read it — server-only by default. `sites` gains nullable latitude/longitude, filled by Google Places autocomplete or typed by hand. A map page renders one pin per mapped site and names the sites that have none.

**Tech Stack:** Go 1.25 + Gin + GORM + PostgreSQL 15; React 18 + TypeScript + Ant Design + React Query; `@vis.gl/react-google-maps` 1.9.0.

**Spec:** `docs/superpowers/specs/2026-08-30-site-mapping-design.md`

## Global Constraints

- Schema changes ship as numbered SQL under `backend/migrations/` **and** the model is registered in `models.AutoMigrate`.
- Credentials are encrypted with the existing `utils.Encrypt(plaintext, key)` / `utils.Decrypt(ciphertext, key)`. No new crypto.
- No file over 350 lines. No function over 50 lines. Maximum 3 levels of nesting.
- Backend gates: `gofmt -s -l .` empty, `go vet ./...` clean, `go test ./... -race` green, `go mod verify` passes.
- Frontend gates: `npm test -- --run`, `npm run lint` (zero errors), `npm run format:check`, `npm run build` — all green.
- UI text on these pages is **English**, matching Dashboard, Sites, and Graphs.
- New dependency allowed in this plan and only this one: `@vis.gl/react-google-maps` pinned at `1.9.0`.
- A setting's value is never written to logs, never to audit rows, and never serialised into an HTTP response except through `GET /api/v1/settings/browser`.
- Comments explain **why**, never what. Go doc comments on exported identifiers are required.

---

## File Structure

**Backend — created**

| File | Responsibility |
|---|---|
| `backend/internal/models/app_setting.go` | The stored row. Value is ciphertext and is `json:"-"`. |
| `backend/internal/models/setting_registry.go` | Which settings exist and who may read each. |
| `backend/migrations/27_add_app_settings.sql` | `app_settings` table. |
| `backend/migrations/28_add_site_coordinates.sql` | `latitude` / `longitude` on `sites`. |
| `backend/internal/services/setting_service.go` | Encrypt, decrypt, mask, list, upsert, delete. |
| `backend/internal/services/setting_service_test.go` | Service behaviour. |
| `backend/internal/api/setting_dto.go` | Request/response shapes. |
| `backend/internal/api/setting_handler.go` | HTTP layer plus audit calls. |
| `backend/internal/api/setting_handler_test.go` | RBAC and payload behaviour. |

**Backend — modified**

| File | Change |
|---|---|
| `backend/internal/models/models.go` | Register `AppSetting` in `AutoMigrate`. |
| `backend/internal/models/site.go` | Add `Latitude`, `Longitude` as `*float64`. |
| `backend/internal/api/site_dto.go` | Coordinates in request and response. |
| `backend/internal/api/site_handler.go` | Pass coordinates through; map `ErrValidation` to 400. |
| `backend/internal/services/site_service.go` | Accept and validate coordinates. |
| `backend/internal/services/site_service_test.go` | Coordinate validation cases. |
| `backend/internal/api/router.go` | Construct the settings service/handler, register routes. |

**Frontend — created**

| File | Responsibility |
|---|---|
| `frontend/src/domain/entities/Setting.ts` | `SettingStatus`, `BrowserSettings`. |
| `frontend/src/infrastructure/repositories/SettingRepository.ts` | HTTP calls. |
| `frontend/src/application/hooks/useSettings.ts` | React Query hooks. |
| `frontend/src/presentation/pages/SettingsPage.tsx` | Admin settings screen. |
| `frontend/src/presentation/components/settings/SettingRow.tsx` | One setting: status, preview, actions. |
| `frontend/src/presentation/components/settings/SettingValueModal.tsx` | Enter or replace a value. |
| `frontend/src/presentation/components/settings/MapsKeyGuidance.tsx` | Permanent restriction instructions. |
| `frontend/src/presentation/components/settings/index.ts` | Barrel. |
| `frontend/src/presentation/components/sites/siteCoordinates.ts` | Pure coordinate parsing and validation. |
| `frontend/src/presentation/components/sites/AddressAutocomplete.tsx` | Places-backed address input with plain-input fallback. |
| `frontend/src/presentation/pages/MapPage.tsx` | Map screen. |
| `frontend/src/presentation/components/map/SiteMap.tsx` | Map plus pins plus info windows. |
| `frontend/src/presentation/components/map/UnmappedSitesPanel.tsx` | Sites with no coordinates. |
| `frontend/src/presentation/components/map/index.ts` | Barrel. |

**Frontend — modified**

| File | Change |
|---|---|
| `frontend/src/infrastructure/http/endpoints.ts` | Settings endpoints. |
| `frontend/src/domain/entities/Site.ts` | Coordinates on `Site`, `CreateSiteDto`, `UpdateSiteDto`. |
| `frontend/src/domain/entities/index.ts` | Export `Setting`. |
| `frontend/src/application/hooks/index.ts` | Export `useSettings`. |
| `frontend/src/presentation/components/sites/SiteModal.tsx` | Address autocomplete plus coordinate fields. |
| `frontend/src/presentation/components/sites/index.ts` | Export the new components. |
| `frontend/src/presentation/components/layout/navigationRoutes.tsx` | `Settings` (admin) and `Map` entries. |
| `frontend/src/presentation/routes/index.tsx` | `/settings` and `/map` routes. |
| `frontend/package.json` | The pinned dependency. |

---

## Task 1: Setting registry and storage

**Files:**
- Create: `backend/internal/models/app_setting.go`
- Create: `backend/internal/models/setting_registry.go`
- Create: `backend/internal/models/setting_registry_test.go`
- Create: `backend/migrations/27_add_app_settings.sql`
- Modify: `backend/internal/models/models.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `models.AppSetting`; `models.SettingVisibility` with `VisibilityServerOnly` / `VisibilityBrowser`; `models.SettingDefinition{Name, Label, Description string; Visibility SettingVisibility}`; `models.SettingDefinitions() []SettingDefinition`; `models.LookupSetting(name string) (SettingDefinition, bool)`; `models.SettingGoogleMapsAPIKey = "google_maps_api_key"`.

- [ ] **Step 1: Write the failing test**

`backend/internal/models/setting_registry_test.go`:

```go
package models

import "testing"

func TestUnstatedVisibilityKeepsAValueOnTheServer(t *testing.T) {
	// A definition added without thinking about visibility must not reach a
	// browser. Exposing a credential has to be something someone typed.
	var forgotten SettingDefinition

	if forgotten.Visibility != VisibilityServerOnly {
		t.Fatalf("zero visibility = %q, want server-only", forgotten.Visibility)
	}
}

func TestRegistryKnowsTheMapsKey(t *testing.T) {
	definition, ok := LookupSetting(SettingGoogleMapsAPIKey)
	if !ok {
		t.Fatal("the Maps key must be a known setting")
	}
	if definition.Visibility != VisibilityBrowser {
		t.Fatalf("visibility = %q, want browser: the map cannot run server-side", definition.Visibility)
	}
	if definition.Label == "" {
		t.Fatal("a setting needs a label the settings page can show")
	}
}

func TestRegistryRejectsAnUnknownName(t *testing.T) {
	if _, ok := LookupSetting("anything_at_all"); ok {
		t.Fatal("the store must not accept names nobody declared")
	}
}

func TestDefinitionsCannotBeMutatedByCallers(t *testing.T) {
	// The registry decides who may read a credential. A caller that could edit
	// the returned slice could quietly widen that.
	first := SettingDefinitions()
	first[0].Visibility = VisibilityBrowser
	first[0].Name = "tampered"

	second := SettingDefinitions()
	if second[0].Name == "tampered" {
		t.Fatal("SettingDefinitions must hand out a copy")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/models/ -run TestRegistry -v`
Expected: FAIL — `undefined: LookupSetting`, `undefined: SettingDefinition`.

- [ ] **Step 3: Write the registry**

`backend/internal/models/setting_registry.go`:

```go
package models

// SettingVisibility says who may read a setting's value.
type SettingVisibility string

const (
	// VisibilityServerOnly keeps a value on the backend. It is deliberately the
	// zero value: a definition that says nothing about visibility does not
	// reach a browser, so exposing a credential takes a decision someone typed
	// rather than one they forgot.
	VisibilityServerOnly SettingVisibility = ""
	// VisibilityBrowser delivers a value to any authenticated user, for
	// features that cannot run anywhere else.
	VisibilityBrowser SettingVisibility = "browser"
)

// SettingGoogleMapsAPIKey drives the site map and address autocomplete.
const SettingGoogleMapsAPIKey = "google_maps_api_key"

// SettingDefinition describes a setting the installation understands.
type SettingDefinition struct {
	Name        string
	Label       string
	Description string
	Visibility  SettingVisibility
}

var settingRegistry = []SettingDefinition{
	{
		Name:        SettingGoogleMapsAPIKey,
		Label:       "Google Maps API key",
		Description: "Enables the site map and address autocomplete. This key is delivered to the browser and cannot be kept secret — restrict it to this site in Google Cloud Console.",
		Visibility:  VisibilityBrowser,
	},
}

// SettingDefinitions returns every known setting. The result is a copy: the
// registry decides who may read a credential, and a caller able to edit it
// could widen that silently.
func SettingDefinitions() []SettingDefinition {
	out := make([]SettingDefinition, len(settingRegistry))
	copy(out, settingRegistry)
	return out
}

// LookupSetting finds a definition by name. Names outside the registry are
// rejected, so the store cannot be filled with arbitrary keys.
func LookupSetting(name string) (SettingDefinition, bool) {
	for _, definition := range settingRegistry {
		if definition.Name == name {
			return definition, true
		}
	}
	return SettingDefinition{}, false
}
```

`backend/internal/models/app_setting.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// AppSetting stores one credential for an external integration. Value is always
// AES-256-GCM ciphertext, and is tagged json:"-" so the model can never be
// serialised into a response by accident.
type AppSetting struct {
	Name      string     `gorm:"primaryKey;size:64" json:"name"`
	Value     string     `gorm:"type:text;not null" json:"-"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid" json:"updated_by,omitempty"`
}

// TableName returns the table name for AppSetting.
func (AppSetting) TableName() string {
	return "app_settings"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/models/ -run "TestRegistry|TestUnstated|TestDefinitions" -v`
Expected: PASS, 4 tests.

- [ ] **Step 5: Add the migration and register the model**

`backend/migrations/27_add_app_settings.sql`:

```sql
-- Credentials for external integrations. Values are AES-256-GCM ciphertext
-- written by the API; nothing else writes to this table.
CREATE TABLE IF NOT EXISTS app_settings (
    name VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID
);
```

In `backend/internal/models/models.go`, add `&AppSetting{},` to the `AutoMigrate` list, after `&WorkerHeartbeat{},`.

- [ ] **Step 6: Verify the whole models package**

Run: `cd backend && gofmt -s -l . && go vet ./internal/models/ && go test ./internal/models/ -v`
Expected: no gofmt output, no vet output, all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/models/app_setting.go backend/internal/models/setting_registry.go backend/internal/models/setting_registry_test.go backend/internal/models/models.go backend/migrations/27_add_app_settings.sql
git commit -m "feat(settings): add the encrypted setting store and its registry"
```

---

## Task 2: Setting service

**Files:**
- Create: `backend/internal/services/setting_service.go`
- Create: `backend/internal/services/setting_service_test.go`

**Interfaces:**
- Consumes: `models.AppSetting`, `models.SettingDefinitions()`, `models.LookupSetting(name)`, `models.VisibilityBrowser`, `models.SettingGoogleMapsAPIKey`, `utils.Encrypt(plaintext, key string) (string, error)`, `utils.Decrypt(ciphertext, key string) (string, error)`.
- Produces:
  - `services.SettingStatus{Name, Label, Description string; Configured bool; Preview string; UpdatedAt *time.Time}`
  - `services.NewSettingService(db *gorm.DB, encryptionKey string) *SettingService`
  - `(*SettingService).List() ([]SettingStatus, error)`
  - `(*SettingService).Set(name, value string, actor uuid.UUID) error`
  - `(*SettingService).Delete(name string) error`
  - `(*SettingService).Value(name string) (string, error)`
  - `(*SettingService).BrowserValues() (map[string]string, error)`
  - `services.ErrUnknownSetting`, `services.ErrSettingNotConfigured`

- [ ] **Step 1: Write the failing test**

`backend/internal/services/setting_service_test.go`:

```go
package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testEncryptionKey = "01234567890123456789012345678901"

func newSettingService(t *testing.T) (*SettingService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}))

	return NewSettingService(db, testEncryptionKey), db
}

func TestSettingIsStoredEncryptedAndReadsBack(t *testing.T) {
	service, db := newSettingService(t)

	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	var stored models.AppSetting
	require.NoError(t, db.First(&stored, "name = ?", models.SettingGoogleMapsAPIKey).Error)
	require.NotContains(t, stored.Value, "AIzaSyTESTKEY123", "the key must not sit in the table as plaintext")

	value, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.NoError(t, err)
	require.Equal(t, "AIzaSyTESTKEY123", value)
}

func TestSettingRejectsANameNobodyDeclared(t *testing.T) {
	service, _ := newSettingService(t)

	err := service.Set("secret_backdoor", "value", uuid.New())
	require.ErrorIs(t, err, ErrUnknownSetting)
}

func TestSettingRejectsABlankValue(t *testing.T) {
	// Storing "" would read as configured on the settings page while the
	// feature stays broken.
	service, _ := newSettingService(t)

	require.Error(t, service.Set(models.SettingGoogleMapsAPIKey, "   ", uuid.New()))
}

func TestSettingReportsNotConfiguredDistinctlyFromAFailure(t *testing.T) {
	service, _ := newSettingService(t)

	_, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.ErrorIs(t, err, ErrSettingNotConfigured)
}

func TestSettingReplacesRatherThanDuplicates(t *testing.T) {
	service, db := newSettingService(t)
	actor := uuid.New()

	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "first-value", actor))
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "second-value", actor))

	var count int64
	require.NoError(t, db.Model(&models.AppSetting{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	value, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.NoError(t, err)
	require.Equal(t, "second-value", value)
}

func TestSettingDeleteRemovesTheValue(t *testing.T) {
	service, _ := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	require.NoError(t, service.Delete(models.SettingGoogleMapsAPIKey))

	_, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.ErrorIs(t, err, ErrSettingNotConfigured)
}

func TestSettingListDescribesEveryKnownSettingEvenUnset(t *testing.T) {
	// The page has to offer a setting before it is configured, or there is no
	// way to configure it.
	service, _ := newSettingService(t)

	statuses, err := service.List()
	require.NoError(t, err)
	require.Len(t, statuses, len(models.SettingDefinitions()))
	require.False(t, statuses[0].Configured)
	require.Empty(t, statuses[0].Preview)
	require.NotEmpty(t, statuses[0].Label)
}

func TestSettingListNeverCarriesTheValue(t *testing.T) {
	service, _ := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	statuses, err := service.List()
	require.NoError(t, err)
	require.True(t, statuses[0].Configured)
	require.NotContains(t, statuses[0].Preview, "SyTESTKEY")
	require.Equal(t, "AIza••••••••Y123", statuses[0].Preview)
	require.NotNil(t, statuses[0].UpdatedAt)
}

func TestSettingPreviewHidesTheLengthOfAShortValue(t *testing.T) {
	// A dot count that tracks the value's length leaks how long the secret is.
	service, _ := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "short", uuid.New()))

	statuses, err := service.List()
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("•", 8), statuses[0].Preview)
}

func TestBrowserValuesCarryOnlyWhatMustRunClientSide(t *testing.T) {
	service, db := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	// A stored row whose name is not a browser definition must not escape, even
	// though it sits in the same table.
	ciphertext, err := encryptForTest("server-secret")
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.AppSetting{Name: "server_only_probe", Value: ciphertext}).Error)

	values, err := service.BrowserValues()
	require.NoError(t, err)
	require.Equal(t, map[string]string{models.SettingGoogleMapsAPIKey: "AIzaSyTESTKEY123"}, values)
}

func TestBrowserValuesAreEmptyBeforeAnythingIsConfigured(t *testing.T) {
	service, _ := newSettingService(t)

	values, err := service.BrowserValues()
	require.NoError(t, err)
	require.Empty(t, values)
}

func TestValueRejectsAnUnknownName(t *testing.T) {
	service, _ := newSettingService(t)

	_, err := service.Value("server_only_probe")
	require.True(t, errors.Is(err, ErrUnknownSetting))
}
```

Add this helper at the bottom of the same test file:

```go
func encryptForTest(plaintext string) (string, error) {
	return utils.Encrypt(plaintext, testEncryptionKey)
}
```

and add `"github.com/tikman/olt-provisioning/internal/utils"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestSetting -v`
Expected: FAIL — `undefined: NewSettingService`.

- [ ] **Step 3: Write the service**

`backend/internal/services/setting_service.go`:

```go
package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrUnknownSetting marks a name that is not in the registry. The store is not
// an arbitrary key-value bag.
var ErrUnknownSetting = errors.New("unknown setting")

// ErrSettingNotConfigured means the setting exists but has no value yet, which
// callers handle by degrading rather than failing.
var ErrSettingNotConfigured = errors.New("setting not configured")

// previewDots is fixed so the mask cannot leak the value's length.
const previewDots = "••••••••"

// previewEdge is how much of a long value stays readable, enough to tell two
// keys apart without revealing either.
const previewEdge = 4

// SettingStatus describes a setting for the management page. It deliberately
// carries no value.
type SettingStatus struct {
	Name        string     `json:"name"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Configured  bool       `json:"configured"`
	Preview     string     `json:"preview"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// SettingService stores credentials for external integrations.
type SettingService struct {
	db            *gorm.DB
	encryptionKey string
}

// NewSettingService creates a setting service.
func NewSettingService(db *gorm.DB, encryptionKey string) *SettingService {
	return &SettingService{db: db, encryptionKey: encryptionKey}
}

// List returns every known setting, configured or not, so the page can offer
// one that has never been set.
func (s *SettingService) List() ([]SettingStatus, error) {
	stored, err := s.storedByName()
	if err != nil {
		return nil, err
	}

	definitions := models.SettingDefinitions()
	statuses := make([]SettingStatus, 0, len(definitions))
	for _, definition := range definitions {
		status := SettingStatus{
			Name:        definition.Name,
			Label:       definition.Label,
			Description: definition.Description,
		}
		if row, ok := stored[definition.Name]; ok {
			plaintext, decryptErr := utils.Decrypt(row.Value, s.encryptionKey)
			if decryptErr != nil {
				return nil, fmt.Errorf("failed to read setting %s: %w", definition.Name, decryptErr)
			}
			updatedAt := row.UpdatedAt
			status.Configured = true
			status.Preview = maskSecret(plaintext)
			status.UpdatedAt = &updatedAt
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// Set stores a value for a known setting, replacing any previous one.
func (s *SettingService) Set(name, value string, actor uuid.UUID) error {
	if _, ok := models.LookupSetting(name); !ok {
		return fmt.Errorf("%w: %s", ErrUnknownSetting, name)
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		// An empty value would show as configured while the feature stays
		// broken, which is worse than not being set at all.
		return fmt.Errorf("%w: a value is required", ErrValidation)
	}

	ciphertext, err := utils.Encrypt(trimmed, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt setting: %w", err)
	}

	setting := models.AppSetting{
		Name:      name,
		Value:     ciphertext,
		UpdatedAt: time.Now(),
		UpdatedBy: &actor,
	}

	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
	}).Create(&setting).Error
}

// Delete removes a stored value. Deleting one that was never set is not an
// error: the caller's intent is already satisfied.
func (s *SettingService) Delete(name string) error {
	if _, ok := models.LookupSetting(name); !ok {
		return fmt.Errorf("%w: %s", ErrUnknownSetting, name)
	}
	return s.db.Where("name = ?", name).Delete(&models.AppSetting{}).Error
}

// Value decrypts a setting for backend use.
func (s *SettingService) Value(name string) (string, error) {
	if _, ok := models.LookupSetting(name); !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownSetting, name)
	}

	var setting models.AppSetting
	err := s.db.First(&setting, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("%w: %s", ErrSettingNotConfigured, name)
	}
	if err != nil {
		return "", fmt.Errorf("failed to read setting: %w", err)
	}

	return utils.Decrypt(setting.Value, s.encryptionKey)
}

// BrowserValues returns the settings whose features can only run client-side.
// It is the only path by which a stored value leaves this server, so it walks
// the registry rather than the table: a row whose name is not a browser
// definition is never read, however it got there.
func (s *SettingService) BrowserValues() (map[string]string, error) {
	stored, err := s.storedByName()
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, definition := range models.SettingDefinitions() {
		if definition.Visibility != models.VisibilityBrowser {
			continue
		}
		row, ok := stored[definition.Name]
		if !ok {
			continue
		}
		plaintext, decryptErr := utils.Decrypt(row.Value, s.encryptionKey)
		if decryptErr != nil {
			return nil, fmt.Errorf("failed to read setting %s: %w", definition.Name, decryptErr)
		}
		values[definition.Name] = plaintext
	}
	return values, nil
}

func (s *SettingService) storedByName() (map[string]models.AppSetting, error) {
	var rows []models.AppSetting
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	byName := make(map[string]models.AppSetting, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}
	return byName, nil
}

// maskSecret keeps the ends of a long value readable so two keys can be told
// apart, and hides everything about a short one.
func maskSecret(value string) string {
	if len(value) < previewEdge*2+4 {
		return previewDots
	}
	return value[:previewEdge] + previewDots + value[len(value)-previewEdge:]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/services/ -run "TestSetting|TestBrowserValues|TestValueRejects" -v`
Expected: PASS, 12 tests.

- [ ] **Step 5: Verify formatting and vet**

Run: `cd backend && gofmt -s -l . && go vet ./internal/services/`
Expected: no output from either.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/setting_service.go backend/internal/services/setting_service_test.go
git commit -m "feat(settings): encrypt, mask, and gate access to stored credentials"
```

---

## Task 3: Settings API

**Files:**
- Create: `backend/internal/api/setting_dto.go`
- Create: `backend/internal/api/setting_handler.go`
- Create: `backend/internal/api/setting_handler_test.go`
- Modify: `backend/internal/api/router.go`

**Interfaces:**
- Consumes: `services.NewSettingService(db, encryptionKey)`, `(*services.SettingService).List/Set/Delete/BrowserValues`, `services.ErrUnknownSetting`, `services.ErrValidation`, `services.AuditService.Log(userID uuid.UUID, action, resourceType string, resourceID uuid.UUID, oldValue, newValue map[string]interface{}, ipAddress, userAgent string) error`, `middleware.GetUserID(c)`, `middleware.RequireRole(models.UserRoleAdmin)`.
- Produces: routes `GET /api/v1/settings`, `GET /api/v1/settings/browser`, `PUT /api/v1/settings/:name`, `DELETE /api/v1/settings/:name`.

- [ ] **Step 1: Write the failing test**

`backend/internal/api/setting_handler_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testEncryptionKey already exists in internal/api/test_helpers.go; do not
// redeclare it.
func setupSettingHandler(t *testing.T) (*SettingHandler, *services.SettingService) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}, &models.AuditLog{}))

	service := services.NewSettingService(db, testEncryptionKey)
	return NewSettingHandler(service, nil), service
}

func settingContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uuid.New())
	c.Set("user_role", models.UserRoleAdmin)
	return c, recorder
}

func TestSettingListNeverReturnsAValue(t *testing.T) {
	handler, service := setupSettingHandler(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	c, recorder := settingContext(t, http.MethodGet, "/api/v1/settings", "")
	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "AIzaSyTESTKEY123")
	require.Contains(t, recorder.Body.String(), "AIza")
}

func TestSettingSetStoresAndReportsStatus(t *testing.T) {
	handler, service := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, `{"value":"AIzaSyTESTKEY123"}`)
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Set(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	value, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.NoError(t, err)
	require.Equal(t, "AIzaSyTESTKEY123", value)
}

func TestSettingSetRefusesAnUnknownName(t *testing.T) {
	handler, _ := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/backdoor", `{"value":"x"}`)
	c.Params = gin.Params{{Key: "name", Value: "backdoor"}}
	handler.Set(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "UNKNOWN_SETTING")
}

func TestSettingSetRefusesABlankValue(t *testing.T) {
	handler, _ := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, `{"value":"   "}`)
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Set(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestSettingBrowserEndpointCarriesOnlyBrowserSettings(t *testing.T) {
	handler, service := setupSettingHandler(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	c, recorder := settingContext(t, http.MethodGet, "/api/v1/settings/browser", "")
	handler.Browser(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, map[string]string{models.SettingGoogleMapsAPIKey: "AIzaSyTESTKEY123"}, body)
}

func TestSettingBrowserEndpointAnswersEmptyBeforeAnythingIsSet(t *testing.T) {
	// "No key configured" is a normal state the frontend renders, not an error.
	handler, _ := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodGet, "/api/v1/settings/browser", "")
	handler.Browser(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{}`, recorder.Body.String())
}

func TestSettingDeleteRemovesTheValue(t *testing.T) {
	handler, service := setupSettingHandler(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	c, recorder := settingContext(t, http.MethodDelete, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, "")
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Delete(c)

	require.Equal(t, http.StatusNoContent, recorder.Code)

	_, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.ErrorIs(t, err, services.ErrSettingNotConfigured)
}

func TestSettingAuditRecordsTheChangeButNotTheValue(t *testing.T) {
	// An audit trail holding the credential defeats the encryption it audits.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}, &models.AuditLog{}))

	service := services.NewSettingService(db, testEncryptionKey)
	handler := NewSettingHandler(service, services.NewAuditService(db, zap.NewNop()))

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, `{"value":"AIzaSyTESTKEY123"}`)
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Set(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var logs []models.AuditLog
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, "setting", logs[0].ResourceType)
	require.NotContains(t, string(logs[0].NewValue), "AIzaSyTESTKEY123")
	require.Contains(t, string(logs[0].NewValue), models.SettingGoogleMapsAPIKey)
}

func TestSettingsAreClosedToNonAdminsButBrowserValuesAreNot(t *testing.T) {
	// Calling a handler directly skips the middleware, so it proves nothing
	// about roles. This goes through the real router.
	logger, _ := zap.NewDevelopment()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	cfg := &config.Config{
		LogLevel:       "debug",
		EncryptionKey:  testEncryptionKey,
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}

	sessionStore := auth.NewMemoryStore(24 * time.Hour)
	token, err := sessionStore.Create(uuid.New(), models.UserRoleViewer)
	require.NoError(t, err)

	router := Setup(gin.New(), cfg, db, sessionStore, logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}))

	call := func(path string) int {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder.Code
	}

	// A viewer must not be able to enumerate stored credentials...
	require.Equal(t, http.StatusForbidden, call("/api/v1/settings"))
	// ...but must receive the values their own browser needs to draw a map.
	require.Equal(t, http.StatusOK, call("/api/v1/settings/browser"))
}
```

The two tests above need these imports added to the file: `"time"`,
`"go.uber.org/zap"`, `"github.com/tikman/olt-provisioning/internal/auth"`,
`"github.com/tikman/olt-provisioning/internal/config"`, and
`"github.com/tikman/olt-provisioning/internal/connectivity"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/api/ -run TestSetting -v`
Expected: FAIL — `undefined: NewSettingHandler`.

- [ ] **Step 3: Write the DTO and handler**

`backend/internal/api/setting_dto.go`:

```go
package api

// SetSettingRequest carries a new value for a setting. The value is never
// echoed back.
type SetSettingRequest struct {
	Value string `json:"value" binding:"required"`
}
```

`backend/internal/api/setting_handler.go`:

```go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

// SettingHandler manages credentials for external integrations.
type SettingHandler struct {
	service      *services.SettingService
	auditService *services.AuditService
}

// NewSettingHandler creates a setting handler.
func NewSettingHandler(service *services.SettingService, auditService *services.AuditService) *SettingHandler {
	return &SettingHandler{service: service, auditService: auditService}
}

// List handles GET /api/v1/settings. It returns status only, never values.
func (h *SettingHandler) List(c *gin.Context) {
	statuses, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "LIST_FAILED",
			Error: "Failed to read settings",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

// Browser handles GET /api/v1/settings/browser. Any authenticated user may
// call it: the values it returns drive features that only run client-side.
func (h *SettingHandler) Browser(c *gin.Context) {
	values, err := h.service.BrowserValues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "READ_FAILED",
			Error: "Failed to read settings",
		})
		return
	}
	c.JSON(http.StatusOK, values)
}

// Set handles PUT /api/v1/settings/:name.
func (h *SettingHandler) Set(c *gin.Context) {
	name := c.Param("name")

	var req SetSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_REQUEST",
			Error: "A value is required",
		})
		return
	}

	actorID, _ := middleware.GetUserID(c)
	if err := h.service.Set(name, req.Value, actorID); err != nil {
		h.reportFailure(c, err)
		return
	}

	h.audit(c, actorID, "update", name)

	statuses, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "LIST_FAILED",
			Error: "Saved, but the status could not be read back",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

// Delete handles DELETE /api/v1/settings/:name.
func (h *SettingHandler) Delete(c *gin.Context) {
	name := c.Param("name")

	if err := h.service.Delete(name); err != nil {
		h.reportFailure(c, err)
		return
	}

	actorID, _ := middleware.GetUserID(c)
	h.audit(c, actorID, "delete", name)

	c.Status(http.StatusNoContent)
}

func (h *SettingHandler) reportFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrUnknownSetting):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "UNKNOWN_SETTING",
			Error: "No such setting",
		})
	case errors.Is(err, services.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_VALUE",
			Error: "A value is required",
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "SAVE_FAILED",
			Error: "Failed to save the setting",
		})
	}
}

// audit records who changed which setting. The value is deliberately absent:
// an audit trail holding the credential defeats the encryption it audits.
// Settings are keyed by name rather than UUID, so the resource id is uuid.Nil.
func (h *SettingHandler) audit(c *gin.Context, actorID uuid.UUID, action, name string) {
	if h.auditService == nil {
		return
	}
	_ = h.auditService.Log(
		actorID,
		action,
		"setting",
		uuid.Nil,
		nil,
		map[string]interface{}{"name": name},
		c.ClientIP(),
		c.Request.UserAgent(),
	)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/api/ -run TestSetting -v`
Expected: PASS, 9 tests.

- [ ] **Step 5: Register the routes**

In `backend/internal/api/router.go`, beside the other service constructions (near `configTemplateService`), add:

```go
	settingService := services.NewSettingService(db, cfg.EncryptionKey)
```

beside the other handler constructions, add:

```go
	settingHandler := NewSettingHandler(settingService, auditService)
```

and after the `sites` route group, add:

```go
		// GET /browser is static while PUT/DELETE take :name. Gin keeps one
		// tree per method so these coexist — but a GET /settings/:name must
		// never be added here without moving /browser, or the router panics on
		// a wildcard conflict.
		settings := api.Group("/settings")
		settings.Use(middleware.AuthMiddleware(authStore, logger))
		{
			settings.GET("/browser", settingHandler.Browser)
			settings.GET("", middleware.RequireRole(models.UserRoleAdmin), settingHandler.List)
			settings.PUT("/:name", middleware.RequireRole(models.UserRoleAdmin), settingHandler.Set)
			settings.DELETE("/:name", middleware.RequireRole(models.UserRoleAdmin), settingHandler.Delete)
		}
```

- [ ] **Step 6: Verify the whole backend**

Run: `cd backend && gofmt -s -l . && go vet ./... && go build ./cmd/api ./cmd/worker && go test ./internal/api/ ./internal/services/ ./internal/models/ -race`
Expected: no gofmt output, no vet output, build succeeds, all packages `ok`.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/api/setting_dto.go backend/internal/api/setting_handler.go backend/internal/api/setting_handler_test.go backend/internal/api/router.go
git commit -m "feat(settings): expose an admin API and a browser-values endpoint"
```

---

## Task 4: Settings page

**Files:**
- Create: `frontend/src/domain/entities/Setting.ts`
- Create: `frontend/src/infrastructure/repositories/SettingRepository.ts`
- Create: `frontend/src/application/hooks/useSettings.ts`
- Create: `frontend/src/presentation/components/settings/SettingRow.tsx`
- Create: `frontend/src/presentation/components/settings/SettingValueModal.tsx`
- Create: `frontend/src/presentation/components/settings/MapsKeyGuidance.tsx`
- Create: `frontend/src/presentation/components/settings/index.ts`
- Create: `frontend/src/presentation/components/settings/SettingRow.test.tsx`
- Create: `frontend/src/presentation/pages/SettingsPage.tsx`
- Create: `frontend/src/presentation/pages/__tests__/SettingsPage.test.tsx`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Modify: `frontend/src/domain/entities/index.ts`
- Modify: `frontend/src/application/hooks/index.ts`
- Modify: `frontend/src/infrastructure/repositories/index.ts`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`
- Modify: `frontend/src/presentation/routes/index.tsx`

**Interfaces:**
- Consumes: `GET /api/v1/settings` → `{data: SettingStatus[]}`; `PUT /api/v1/settings/:name` body `{value}`; `DELETE /api/v1/settings/:name`; `GET /api/v1/settings/browser` → `Record<string,string>`.
- Produces: `SettingStatus`, `BrowserSettings`, `useSettings()`, `useSaveSetting()`, `useDeleteSetting()`, `useBrowserSettings()`, `useGoogleMapsKey()`, `GOOGLE_MAPS_API_KEY` constant.

- [ ] **Step 1: Write the failing test**

`frontend/src/presentation/components/settings/SettingRow.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SettingRow } from "./SettingRow";

const CONFIGURED = {
  name: "google_maps_api_key",
  label: "Google Maps API key",
  description: "Enables the site map.",
  configured: true,
  preview: "AIza••••••••Y123",
  updatedAt: "2026-08-30T02:00:00.000Z",
};

describe("SettingRow", () => {
  it("shows the masked preview and never a full value", () => {
    render(
      <SettingRow setting={CONFIGURED} onEdit={vi.fn()} onDelete={vi.fn()} />,
    );

    expect(screen.getByText("Google Maps API key")).toBeInTheDocument();
    expect(screen.getByText("AIza••••••••Y123")).toBeInTheDocument();
  });

  it("says a setting is not configured rather than showing an empty box", () => {
    render(
      <SettingRow
        setting={{ ...CONFIGURED, configured: false, preview: "", updatedAt: undefined }}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(screen.getByText("Not configured")).toBeInTheDocument();
  });

  it("offers removal only for a setting that has a value", () => {
    // A delete button on an unset setting does nothing and invites a click.
    const { rerender } = render(
      <SettingRow setting={CONFIGURED} onEdit={vi.fn()} onDelete={vi.fn()} />,
    );
    expect(screen.getByRole("button", { name: /remove/i })).toBeInTheDocument();

    rerender(
      <SettingRow
        setting={{ ...CONFIGURED, configured: false, preview: "" }}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /remove/i }),
    ).not.toBeInTheDocument();
  });

  it("asks to edit the setting it was given", async () => {
    const onEdit = vi.fn();
    render(<SettingRow setting={CONFIGURED} onEdit={onEdit} onDelete={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: /replace/i }));

    expect(onEdit).toHaveBeenCalledWith(CONFIGURED);
  });
});
```

`frontend/src/presentation/pages/__tests__/SettingsPage.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { App } from "antd";
import SettingsPage from "../SettingsPage";

// SettingsPage takes message and modal from App.useApp(). Rendering it bare
// falls back to antd's statics and warns, so the provider goes in here.
const renderPage = () =>
  render(
    <App>
      <SettingsPage />
    </App>,
  );

const state: {
  data: unknown;
  isLoading: boolean;
} = { data: [], isLoading: false };

vi.mock("@/application/hooks", () => ({
  useSettings: () => state,
  useSaveSetting: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteSetting: () => ({ mutate: vi.fn(), isPending: false }),
}));

describe("SettingsPage", () => {
  beforeEach(() => {
    state.data = [
      {
        name: "google_maps_api_key",
        label: "Google Maps API key",
        description: "Enables the site map.",
        configured: false,
        preview: "",
      },
    ];
    state.isLoading = false;
  });

  it("lists every known setting, including ones never configured", () => {
    renderPage();

    expect(screen.getByText("Google Maps API key")).toBeInTheDocument();
    expect(screen.getByText("Not configured")).toBeInTheDocument();
  });

  it("keeps the key restriction steps on screen beside the Maps key", () => {
    // An operator who believes the key is secret will not go and restrict it,
    // so this guidance is permanent rather than a dismissible hint.
    renderPage();

    expect(screen.getByText(/Application restrictions/i)).toBeInTheDocument();
    expect(screen.getByText(/noc\.radpro\.id/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npm test -- --run src/presentation/components/settings src/presentation/pages/__tests__/SettingsPage.test.tsx`
Expected: FAIL — modules not found.

- [ ] **Step 3: Write the entity, endpoints, repository and hooks**

`frontend/src/domain/entities/Setting.ts`:

```ts
/** One known setting and whether it has a value. Never carries the value. */
export interface SettingStatus {
  name: string;
  label: string;
  description: string;
  configured: boolean;
  /** Masked, e.g. "AIza••••••••Y123". Empty when not configured. */
  preview: string;
  updatedAt?: string;
}

/** Values whose features can only run in the browser, keyed by setting name. */
export type BrowserSettings = Record<string, string>;

export const GOOGLE_MAPS_API_KEY = "google_maps_api_key";
```

Add to `frontend/src/domain/entities/index.ts`:

```ts
export * from "./Setting";
```

Add to `frontend/src/infrastructure/http/endpoints.ts`, after the Sites block:

```ts
  // Settings
  SETTINGS: "/api/v1/settings",
  SETTINGS_BROWSER: "/api/v1/settings/browser",
  SETTING_BY_NAME: (name: string) => `/api/v1/settings/${name}`,
```

`frontend/src/infrastructure/repositories/SettingRepository.ts`:

```ts
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { BrowserSettings, SettingStatus } from "@/domain/entities";

export class SettingRepository {
  async list(): Promise<SettingStatus[]> {
    const response = await apiClient.get(API_ENDPOINTS.SETTINGS);
    return response.data.data ?? [];
  }

  async save(name: string, value: string): Promise<SettingStatus[]> {
    const response = await apiClient.put(API_ENDPOINTS.SETTING_BY_NAME(name), {
      value,
    });
    return response.data.data ?? [];
  }

  async remove(name: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.SETTING_BY_NAME(name));
  }

  // Answers {} when nothing is configured, which is a normal state rather than
  // an error, so this must not throw on an empty installation.
  async browser(): Promise<BrowserSettings> {
    const response = await apiClient.get(API_ENDPOINTS.SETTINGS_BROWSER);
    return response.data ?? {};
  }
}
```

Add to `frontend/src/infrastructure/repositories/index.ts`:

```ts
export * from "./SettingRepository";
```

`frontend/src/application/hooks/useSettings.ts`:

```ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { SettingRepository } from "@/infrastructure/repositories";
import { GOOGLE_MAPS_API_KEY } from "@/domain/entities";

const settingRepository = new SettingRepository();

// Credentials change rarely, and a page that needs the key reads it once on
// entry. Polling it would be traffic for nothing.
const BROWSER_SETTINGS_STALE_TIME = 300_000;

export function useSettings() {
  return useQuery({
    queryKey: ["settings"],
    queryFn: () => settingRepository.list(),
  });
}

export function useSaveSetting() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, value }: { name: string; value: string }) =>
      settingRepository.save(name, value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      queryClient.invalidateQueries({ queryKey: ["settings", "browser"] });
    },
  });
}

export function useDeleteSetting() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => settingRepository.remove(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      queryClient.invalidateQueries({ queryKey: ["settings", "browser"] });
    },
  });
}

export function useBrowserSettings() {
  return useQuery({
    queryKey: ["settings", "browser"],
    queryFn: () => settingRepository.browser(),
    staleTime: BROWSER_SETTINGS_STALE_TIME,
  });
}

/** The Maps key, or undefined when none is configured. */
export function useGoogleMapsKey(): { key?: string; isLoading: boolean } {
  const { data, isLoading } = useBrowserSettings();
  return { key: data?.[GOOGLE_MAPS_API_KEY] || undefined, isLoading };
}
```

Add to `frontend/src/application/hooks/index.ts`:

```ts
export * from "./useSettings";
```

- [ ] **Step 4: Write the components and page**

`frontend/src/presentation/components/settings/SettingRow.tsx`:

```tsx
import { Button, Space, Typography } from "antd";
import type { SettingStatus } from "@/domain/entities";
import { colors } from "@/shared/theme";

interface SettingRowProps {
  setting: SettingStatus;
  onEdit: (setting: SettingStatus) => void;
  onDelete: (setting: SettingStatus) => void;
}

export function SettingRow({ setting, onEdit, onDelete }: SettingRowProps) {
  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        gap: 16,
        padding: "14px 0",
        borderTop: `1px solid ${colors.border}`,
      }}
    >
      <div style={{ minWidth: 0 }}>
        <div style={{ color: colors.textPrimary, fontSize: 14 }}>
          {setting.label}
        </div>
        <div style={{ color: colors.textSecondary, fontSize: 12, marginTop: 4 }}>
          {setting.description}
        </div>
        <div style={{ marginTop: 8 }}>
          {setting.configured ? (
            <Typography.Text code>{setting.preview}</Typography.Text>
          ) : (
            <span style={{ color: colors.textMuted, fontSize: 12 }}>
              Not configured
            </span>
          )}
        </div>
      </div>

      <Space>
        <Button size="small" onClick={() => onEdit(setting)}>
          {setting.configured ? "Replace" : "Set value"}
        </Button>
        {/* Offered only when there is something to remove: a button that does
            nothing still invites a click. */}
        {setting.configured && (
          <Button size="small" danger onClick={() => onDelete(setting)}>
            Remove
          </Button>
        )}
      </Space>
    </div>
  );
}
```

`frontend/src/presentation/components/settings/SettingValueModal.tsx`:

```tsx
import { useEffect, useState } from "react";
import { Input, Modal, Typography } from "antd";
import type { SettingStatus } from "@/domain/entities";

interface SettingValueModalProps {
  setting: SettingStatus | null;
  loading: boolean;
  onClose: () => void;
  onSubmit: (value: string) => void;
}

export function SettingValueModal({
  setting,
  loading,
  onClose,
  onSubmit,
}: SettingValueModalProps) {
  const [value, setValue] = useState("");

  useEffect(() => {
    // The stored value is never sent to this page, so the field always starts
    // empty rather than pretending to hold what is saved.
    setValue("");
  }, [setting]);

  return (
    <Modal
      open={!!setting}
      title={setting?.label}
      okText="Save"
      okButtonProps={{ disabled: !value.trim() }}
      confirmLoading={loading}
      onOk={() => onSubmit(value.trim())}
      onCancel={onClose}
      destroyOnClose
    >
      <Typography.Paragraph type="secondary">
        {setting?.description}
      </Typography.Paragraph>
      <Input.Password
        autoFocus
        value={value}
        placeholder="Paste the value"
        onChange={(event) => setValue(event.target.value)}
        onPressEnter={() => value.trim() && onSubmit(value.trim())}
      />
    </Modal>
  );
}
```

`frontend/src/presentation/components/settings/MapsKeyGuidance.tsx`:

```tsx
import { Alert } from "antd";

/**
 * Permanent, not dismissible. The Maps key is delivered to every browser that
 * loads the map, so the only thing protecting it is a restriction set in Google
 * Cloud Console — and an operator who believes the key is secret will never go
 * and set one.
 */
export function MapsKeyGuidance() {
  return (
    <Alert
      type="warning"
      showIcon
      style={{ marginTop: 12 }}
      message="This key is visible to anyone who opens the map"
      description={
        <ol style={{ margin: "8px 0 0 18px", padding: 0 }}>
          <li>Google Cloud Console → APIs &amp; Services → Credentials</li>
          <li>Open the key → Application restrictions → Websites</li>
          <li>
            Add <code>https://noc.radpro.id/*</code>
          </li>
          <li>
            API restrictions → restrict to Maps JavaScript API and Places API
          </li>
        </ol>
      }
    />
  );
}
```

`frontend/src/presentation/components/settings/index.ts`:

```ts
export * from "./MapsKeyGuidance";
export * from "./SettingRow";
export * from "./SettingValueModal";
```

`frontend/src/presentation/pages/SettingsPage.tsx`:

```tsx
import { useState } from "react";
import { App, Skeleton } from "antd";
import { useDeleteSetting, useSaveSetting, useSettings } from "@/application/hooks";
import { GOOGLE_MAPS_API_KEY, type SettingStatus } from "@/domain/entities";
import { PageHeader, DarkCard } from "../components/common";
import {
  MapsKeyGuidance,
  SettingRow,
  SettingValueModal,
} from "../components/settings";

export default function SettingsPage() {
  const { message, modal } = App.useApp();
  const { data: settings, isLoading } = useSettings();
  const saveMutation = useSaveSetting();
  const deleteMutation = useDeleteSetting();
  const [editing, setEditing] = useState<SettingStatus | null>(null);

  const handleSave = (value: string) => {
    if (!editing) return;
    saveMutation.mutate(
      { name: editing.name, value },
      {
        onSuccess: () => {
          message.success(`${editing.label} saved`);
          setEditing(null);
        },
        onError: (error) => message.error(error.message),
      },
    );
  };

  const handleDelete = (setting: SettingStatus) => {
    modal.confirm({
      title: `Remove ${setting.label}?`,
      content: "Anything relying on it stops working until it is set again.",
      okText: "Remove",
      okButtonProps: { danger: true },
      onOk: () =>
        deleteMutation.mutateAsync(setting.name).then(
          () => message.success(`${setting.label} removed`),
          (error: Error) => message.error(error.message),
        ),
    });
  };

  return (
    <div>
      <PageHeader
        title="Settings"
        description="Credentials for external integrations"
      />

      <DarkCard>
        {isLoading ? (
          <Skeleton active paragraph={{ rows: 3 }} title={false} />
        ) : (
          (settings ?? []).map((setting) => (
            <div key={setting.name}>
              <SettingRow
                setting={setting}
                onEdit={setEditing}
                onDelete={handleDelete}
              />
              {setting.name === GOOGLE_MAPS_API_KEY && <MapsKeyGuidance />}
            </div>
          ))
        )}
      </DarkCard>

      <SettingValueModal
        setting={editing}
        loading={saveMutation.isPending}
        onClose={() => setEditing(null)}
        onSubmit={handleSave}
      />
    </div>
  );
}
```

- [ ] **Step 5: Add the navigation entry and the route**

In `frontend/src/presentation/components/layout/navigationRoutes.tsx`, add `SettingOutlined` to the icon imports and extend the admin-only block so it reads:

```tsx
    ...(role === UserRole.ADMIN
      ? [
          { path: "/users", name: "Users", icon: <UserOutlined /> },
          { path: "/settings", name: "Settings", icon: <SettingOutlined /> },
        ]
      : []),
```

Add this case to `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`:

```tsx
  it("shows Settings only to an admin, since it holds credentials", () => {
    expect(paths(UserRole.ADMIN)).toContain("/settings");
    expect(paths(UserRole.TECHNICIAN)).not.toContain("/settings");
    expect(paths(undefined)).not.toContain("/settings");
  });
```

In `frontend/src/presentation/routes/index.tsx`, add `import SettingsPage from "../pages/SettingsPage";` beside the other page imports, and add this route object after the `vpn` entry:

```tsx
          {
            path: "settings",
            element: <SettingsPage />,
          },
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd frontend && npm test -- --run src/presentation/components/settings src/presentation/pages/__tests__/SettingsPage.test.tsx`
Expected: PASS, 6 tests.

- [ ] **Step 7: Run the full frontend gate**

Run: `cd frontend && npm run format && npm test -- --run && npm run lint && npm run format:check && npm run build`
Expected: all tests pass, zero lint errors, formatting clean, build succeeds.

- [ ] **Step 8: Commit**

```bash
git add frontend/src
git commit -m "feat(settings): add the admin settings page"
```

---

## Task 5: Site coordinates in the backend

**Files:**
- Create: `backend/migrations/28_add_site_coordinates.sql`
- Modify: `backend/internal/models/site.go`
- Modify: `backend/internal/services/site_service.go`
- Modify: `backend/internal/services/site_service_test.go`
- Modify: `backend/internal/api/site_dto.go`
- Modify: `backend/internal/api/site_handler.go`

**Interfaces:**
- Consumes: `services.ErrValidation` (already defined in `wireguard_validate.go`, same package).
- Produces: `models.Site.Latitude *float64`, `models.Site.Longitude *float64`; `(*SiteService).Create(name, location, description string, latitude, longitude *float64) (*models.Site, error)`; `SiteResponse.Latitude`/`.Longitude` as `*float64` with JSON keys `latitude` / `longitude`.

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/services/site_service_test.go`:

```go
func floatPtr(v float64) *float64 { return &v }

func TestSiteAcceptsAValidCoordinatePair(t *testing.T) {
	service, _ := newSiteServiceForTest(t)

	site, err := service.Create("Depok", "Jl. Margonda", "", floatPtr(-6.4025), floatPtr(106.7942))
	require.NoError(t, err)
	require.NotNil(t, site.Latitude)
	require.InDelta(t, -6.4025, *site.Latitude, 0.0001)
	require.InDelta(t, 106.7942, *site.Longitude, 0.0001)
}

func TestSiteSavesWithoutCoordinates(t *testing.T) {
	// Not every site can be placed, and a site must never become unsavable
	// because a location could not be resolved.
	service, _ := newSiteServiceForTest(t)

	site, err := service.Create("Gudang", "Belakang kantor", "", nil, nil)
	require.NoError(t, err)
	require.Nil(t, site.Latitude)
	require.Nil(t, site.Longitude)
}

func TestSiteRefusesHalfACoordinate(t *testing.T) {
	// One value alone is not a partial answer: it would place a pin on the
	// equator or the prime meridian and look deliberate.
	service, _ := newSiteServiceForTest(t)

	_, err := service.Create("Depok", "", "", floatPtr(-6.4), nil)
	require.ErrorIs(t, err, ErrValidation)

	_, err = service.Create("Depok", "", "", nil, floatPtr(106.8))
	require.ErrorIs(t, err, ErrValidation)
}

func TestSiteRefusesCoordinatesOutsideTheGlobe(t *testing.T) {
	service, _ := newSiteServiceForTest(t)

	_, err := service.Create("Nowhere", "", "", floatPtr(91), floatPtr(0))
	require.ErrorIs(t, err, ErrValidation)

	_, err = service.Create("Nowhere", "", "", floatPtr(0), floatPtr(181))
	require.ErrorIs(t, err, ErrValidation)
}

func TestSiteUpdateValidatesCoordinatesToo(t *testing.T) {
	service, _ := newSiteServiceForTest(t)
	site, err := service.Create("Depok", "", "", nil, nil)
	require.NoError(t, err)

	err = service.Update(site.ID, map[string]interface{}{
		"latitude":  floatPtr(-6.4),
		"longitude": floatPtr(200.0),
	})
	require.ErrorIs(t, err, ErrValidation)
}
```

If `newSiteServiceForTest` does not already exist in that file, add it:

```go
func newSiteServiceForTest(t *testing.T) (*SiteService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Site{}))

	return NewSiteService(db), db
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/services/ -run TestSite -v`
Expected: FAIL — `Create` takes 3 arguments, not 5.

- [ ] **Step 3: Add the columns and validation**

`backend/migrations/28_add_site_coordinates.sql`:

```sql
-- Nullable on purpose: existing sites have no coordinates, and a site that
-- cannot be placed on the map must still be a valid site.
ALTER TABLE sites ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;
```

In `backend/internal/models/site.go`, add to the `Site` struct after `Description`:

```go
	Latitude    *float64 `gorm:"type:double precision" json:"latitude,omitempty"`
	Longitude   *float64 `gorm:"type:double precision" json:"longitude,omitempty"`
```

In `backend/internal/services/site_service.go`, change `Create` and add validation:

```go
func (s *SiteService) Create(name, location, description string, latitude, longitude *float64) (*models.Site, error) {
	if err := validateCoordinates(latitude, longitude); err != nil {
		return nil, err
	}

	site := &models.Site{
		Name:        strings.TrimSpace(name),
		Location:    strings.TrimSpace(location),
		Description: strings.TrimSpace(description),
		Latitude:    latitude,
		Longitude:   longitude,
	}

	if err := s.db.Create(site).Error; err != nil {
		return nil, fmt.Errorf("failed to create site: %w", err)
	}

	return site, nil
}

// validateCoordinates rejects a point that cannot exist and a pair that is only
// half given. A lone latitude is not partial data: it would place a pin on the
// prime meridian and look like a deliberate answer.
func validateCoordinates(latitude, longitude *float64) error {
	if (latitude == nil) != (longitude == nil) {
		return fmt.Errorf("%w: latitude and longitude must be given together, or both left empty", ErrValidation)
	}
	if latitude == nil {
		return nil
	}
	if *latitude < -90 || *latitude > 90 {
		return fmt.Errorf("%w: latitude %v is outside -90..90", ErrValidation, *latitude)
	}
	if *longitude < -180 || *longitude > 180 {
		return fmt.Errorf("%w: longitude %v is outside -180..180", ErrValidation, *longitude)
	}
	return nil
}
```

In the same file, at the top of `Update`, add:

```go
	latitude, hasLatitude := updates["latitude"].(*float64)
	longitude, hasLongitude := updates["longitude"].(*float64)
	if hasLatitude || hasLongitude {
		if err := validateCoordinates(latitude, longitude); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/services/ -run TestSite -v`
Expected: PASS.

- [ ] **Step 5: Carry coordinates through the API**

In `backend/internal/api/site_dto.go`, add to `CreateSiteRequest`:

```go
	Latitude    *float64 `json:"latitude" binding:"omitempty,min=-90,max=90"`
	Longitude   *float64 `json:"longitude" binding:"omitempty,min=-180,max=180"`
```

to `UpdateSiteRequest`:

```go
	Latitude    *float64 `json:"latitude" binding:"omitempty,min=-90,max=90"`
	Longitude   *float64 `json:"longitude" binding:"omitempty,min=-180,max=180"`
```

to `SiteResponse`:

```go
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
```

and inside `ToSiteResponse`:

```go
		Latitude:    site.Latitude,
		Longitude:   site.Longitude,
```

In `backend/internal/api/site_handler.go`, change the `Create` call to:

```go
	site, err := h.service.Create(req.Name, req.Location, req.Description, req.Latitude, req.Longitude)
	if err != nil {
		if errors.Is(err, services.ErrValidation) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: strings.TrimPrefix(err.Error(), services.ErrValidation.Error()+": "),
				Code:  "INVALID_COORDINATES",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create site",
			Code:  "CREATE_FAILED",
		})
		return
	}
```

adding `"errors"`, `"strings"`, and `"github.com/tikman/olt-provisioning/internal/services"` to its imports if absent. Apply the same `ErrValidation` branch in `Update`, where the update map gains:

```go
	if req.Latitude != nil {
		updates["latitude"] = req.Latitude
	}
	if req.Longitude != nil {
		updates["longitude"] = req.Longitude
	}
```

- [ ] **Step 6: Verify the whole backend**

Run: `cd backend && gofmt -s -l . && go vet ./... && go build ./cmd/api ./cmd/worker && go test ./... -race`
Expected: no gofmt output, no vet output, build succeeds, every package `ok`.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/models/site.go backend/internal/services/site_service.go backend/internal/services/site_service_test.go backend/internal/api/site_dto.go backend/internal/api/site_handler.go backend/migrations/28_add_site_coordinates.sql
git commit -m "feat(sites): store coordinates, validated as a pair"
```

---

## Task 6: Coordinates in the site form

**Files:**
- Create: `frontend/src/presentation/components/sites/siteCoordinates.ts`
- Create: `frontend/src/presentation/components/sites/siteCoordinates.test.ts`
- Create: `frontend/src/presentation/components/sites/SiteModal.test.tsx`
- Modify: `frontend/src/domain/entities/Site.ts`
- Modify: `frontend/src/presentation/components/sites/SiteModal.tsx`

**Interfaces:**
- Consumes: `SiteResponse` fields `latitude` / `longitude` (camelCase after humps: `latitude`, `longitude`).
- Produces: `parseCoordinate(input: string): number | null`; `coordinateError(latitude: string, longitude: string): string | null`; `Site.latitude?: number`, `Site.longitude?: number` and the same optional pair on both DTOs.

- [ ] **Step 1: Write the failing test**

`frontend/src/presentation/components/sites/siteCoordinates.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { coordinateError, parseCoordinate } from "./siteCoordinates";

describe("parseCoordinate", () => {
  it("reads a decimal degree with either sign", () => {
    expect(parseCoordinate("-6.4025")).toBeCloseTo(-6.4025);
    expect(parseCoordinate("106.7942")).toBeCloseTo(106.7942);
  });

  it("tolerates the spaces a paste brings with it", () => {
    expect(parseCoordinate("  -6.4025 ")).toBeCloseTo(-6.4025);
  });

  it("is null for anything that is not a number", () => {
    expect(parseCoordinate("")).toBeNull();
    expect(parseCoordinate("north")).toBeNull();
    expect(parseCoordinate("6,4025")).toBeNull();
  });
});

describe("coordinateError", () => {
  it("accepts both fields empty, because a site need not be mapped", () => {
    expect(coordinateError("", "")).toBeNull();
  });

  it("accepts a valid pair", () => {
    expect(coordinateError("-6.4025", "106.7942")).toBeNull();
  });

  it("refuses one without the other", () => {
    // Half a coordinate would put a pin on the prime meridian and look
    // deliberate.
    expect(coordinateError("-6.4025", "")).toMatch(/together/i);
    expect(coordinateError("", "106.7942")).toMatch(/together/i);
  });

  it("refuses a point that cannot exist", () => {
    expect(coordinateError("91", "0")).toMatch(/-90/);
    expect(coordinateError("0", "181")).toMatch(/-180/);
  });

  it("refuses text that is not a number at all", () => {
    expect(coordinateError("north", "106.7942")).toMatch(/number/i);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npm test -- --run src/presentation/components/sites/siteCoordinates.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the helper**

`frontend/src/presentation/components/sites/siteCoordinates.ts`:

```ts
/** Reads a decimal degree, or null when the text is not one. */
export function parseCoordinate(input: string): number | null {
  const trimmed = input.trim();
  if (trimmed === "") {
    return null;
  }
  // Number("") is 0 and Number("6,4025") is NaN; the explicit pattern keeps a
  // comma decimal from silently becoming a different place.
  if (!/^[+-]?\d+(\.\d+)?$/.test(trimmed)) {
    return null;
  }
  return Number(trimmed);
}

/**
 * Returns the reason a coordinate pair cannot be saved, or null when it can.
 * Both empty is allowed: not every site can be placed, and a site must never
 * become unsavable because a location could not be resolved.
 */
export function coordinateError(
  latitude: string,
  longitude: string,
): string | null {
  const hasLatitude = latitude.trim() !== "";
  const hasLongitude = longitude.trim() !== "";

  if (!hasLatitude && !hasLongitude) {
    return null;
  }
  if (hasLatitude !== hasLongitude) {
    return "Latitude and longitude must be given together, or both left empty";
  }

  const lat = parseCoordinate(latitude);
  const lng = parseCoordinate(longitude);
  if (lat === null || lng === null) {
    return "Coordinates must be a number, for example -6.4025";
  }
  if (lat < -90 || lat > 90) {
    return "Latitude must be between -90 and 90";
  }
  if (lng < -180 || lng > 180) {
    return "Longitude must be between -180 and 180";
  }
  return null;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npm test -- --run src/presentation/components/sites/siteCoordinates.test.ts`
Expected: PASS, 9 tests.

- [ ] **Step 5: Write the modal test**

`frontend/src/presentation/components/sites/SiteModal.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SiteModal } from "./SiteModal";

describe("SiteModal", () => {
  it("saves a site typed with coordinates by hand", async () => {
    // Places does not know a POP down a gang or a tower in a field, which is a
    // large share of these sites. Manual entry is the path that always works.
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.type(screen.getByLabelText("Latitude"), "-6.4025");
    await userEvent.type(screen.getByLabelText("Longitude"), "106.7942");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "Gudang",
        latitude: -6.4025,
        longitude: 106.7942,
      }),
    );
  });

  it("saves a site with no coordinates at all", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Gudang" }),
    );
    expect(onSubmit.mock.calls[0][0].latitude).toBeUndefined();
  });

  it("refuses half a coordinate and says why", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.type(screen.getByLabelText("Latitude"), "-6.4025");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(await screen.findByText(/together/i)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 6: Extend the entity and the modal**

In `frontend/src/domain/entities/Site.ts`, add `latitude?: number;` and `longitude?: number;` to `Site`, `CreateSiteDto`, and `UpdateSiteDto`.

Replace `frontend/src/presentation/components/sites/SiteModal.tsx` with:

```tsx
import { Modal, Form, Input, Row, Col } from "antd";
import {
  type Site,
  type CreateSiteDto,
  type UpdateSiteDto,
} from "@/domain/entities";
import { useEffect } from "react";
import { coordinateError, parseCoordinate } from "./siteCoordinates";

interface SiteModalProps {
  open: boolean;
  site?: Site;
  onClose: () => void;
  onSubmit: (data: CreateSiteDto | UpdateSiteDto) => void;
  loading: boolean;
}

interface SiteFormValues {
  name: string;
  location?: string;
  description?: string;
  latitude?: string;
  longitude?: string;
}

export function SiteModal({
  open,
  site,
  onClose,
  onSubmit,
  loading,
}: SiteModalProps) {
  const [form] = Form.useForm<SiteFormValues>();

  useEffect(() => {
    if (site) {
      form.setFieldsValue({
        name: site.name,
        location: site.location,
        description: site.description,
        latitude: site.latitude?.toString() ?? "",
        longitude: site.longitude?.toString() ?? "",
      });
    } else {
      form.resetFields();
    }
  }, [site, form]);

  const handleSubmit = () => {
    form
      .validateFields()
      .then((values) => {
        const latitude = parseCoordinate(values.latitude ?? "");
        const longitude = parseCoordinate(values.longitude ?? "");

        onSubmit({
          name: values.name,
          location: values.location,
          description: values.description,
          ...(latitude !== null && longitude !== null
            ? { latitude, longitude }
            : {}),
        });
      })
      // antd renders each failure against its own field, so there is nothing
      // left to report — but without this the rejection escapes unhandled.
      .catch(() => undefined);
  };

  return (
    <Modal
      title={site ? "Edit Site" : "Create Site"}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="name"
          label="Site Name"
          rules={[{ required: true, message: "Please enter site name" }]}
        >
          <Input />
        </Form.Item>

        <Form.Item name="location" label="Location">
          <Input />
        </Form.Item>

        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="latitude"
              label="Latitude"
              dependencies={["longitude"]}
              rules={[
                ({ getFieldValue }) => ({
                  validator: () => {
                    const error = coordinateError(
                      getFieldValue("latitude") ?? "",
                      getFieldValue("longitude") ?? "",
                    );
                    return error ? Promise.reject(new Error(error)) : Promise.resolve();
                  },
                }),
              ]}
            >
              <Input placeholder="-6.4025" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="longitude" label="Longitude">
              <Input placeholder="106.7942" />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item name="description" label="Description">
          <Input.TextArea rows={4} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd frontend && npm test -- --run src/presentation/components/sites`
Expected: PASS, all site component tests.

- [ ] **Step 8: Run the full frontend gate**

Run: `cd frontend && npm run format && npm test -- --run && npm run lint && npm run format:check && npm run build`
Expected: all green.

- [ ] **Step 9: Commit**

```bash
git add frontend/src
git commit -m "feat(sites): enter coordinates by hand, validated as a pair"
```

---

## Task 7: Address autocomplete

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/src/presentation/components/sites/AddressAutocomplete.tsx`
- Create: `frontend/src/presentation/components/sites/AddressAutocomplete.test.tsx`
- Modify: `frontend/src/presentation/components/sites/SiteModal.tsx`
- Modify: `frontend/src/presentation/components/sites/index.ts`

**Interfaces:**
- Consumes: `useGoogleMapsKey()` from Task 4; `coordinateError`, `parseCoordinate` from Task 6.
- Produces: `<AddressAutocomplete value={string} onChange={(value: string) => void} onResolved={(place: {address: string; latitude: number; longitude: number}) => void} />`.

- [ ] **Step 1: Install the dependency**

Run: `cd frontend && npm install --save-exact @vis.gl/react-google-maps@1.9.0`
Expected: `package.json` gains `"@vis.gl/react-google-maps": "1.9.0"` — an exact pin, no caret.

- [ ] **Step 2: Write the failing test**

`frontend/src/presentation/components/sites/AddressAutocomplete.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { AddressAutocomplete } from "./AddressAutocomplete";

const mapsKey: { key?: string; isLoading: boolean } = {
  key: undefined,
  isLoading: false,
};

vi.mock("@/application/hooks", () => ({
  useGoogleMapsKey: () => mapsKey,
}));

// Without this, APIProvider would try to fetch Google's script under jsdom.
vi.mock("@vis.gl/react-google-maps", () => ({
  APIProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useMapsLibrary: () => null,
}));

describe("AddressAutocomplete", () => {
  beforeEach(() => {
    mapsKey.key = undefined;
    mapsKey.isLoading = false;
  });

  it("is a plain text field when no Maps key is configured", async () => {
    // The form must not break, nag, or block saving because a credential is
    // missing.
    const onChange = vi.fn();
    render(
      <AddressAutocomplete value="" onChange={onChange} onResolved={vi.fn()} />,
    );

    await userEvent.type(screen.getByRole("textbox"), "Jl. Margonda");

    expect(onChange).toHaveBeenCalled();
    expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
  });

  it("keeps what the operator typed when the key is present", async () => {
    mapsKey.key = "AIzaSyTESTKEY123";
    const onChange = vi.fn();

    render(
      <AddressAutocomplete value="" onChange={onChange} onResolved={vi.fn()} />,
    );

    await userEvent.type(screen.getByRole("textbox"), "Jl");

    expect(onChange).toHaveBeenCalled();
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd frontend && npm test -- --run src/presentation/components/sites/AddressAutocomplete.test.tsx`
Expected: FAIL — module not found.

- [ ] **Step 4: Write the component**

`frontend/src/presentation/components/sites/AddressAutocomplete.tsx`:

```tsx
import { useEffect, useRef } from "react";
import { Input } from "antd";
import type { InputRef } from "antd";
import { APIProvider, useMapsLibrary } from "@vis.gl/react-google-maps";
import { useGoogleMapsKey } from "@/application/hooks";

export interface ResolvedPlace {
  address: string;
  latitude: number;
  longitude: number;
}

interface AddressAutocompleteProps {
  // Injected by the Form.Item that wraps this field; absent when it is used
  // outside a form.
  value?: string;
  onChange?: (value: string) => void;
  onResolved: (place: ResolvedPlace) => void;
}

/**
 * An address field that suggests real places when a Maps key is configured and
 * is an ordinary text input when one is not.
 *
 * Suggestions are a convenience, never a requirement: Places does not know a
 * POP down a gang or a tower in a field, and if Google were the only way to set
 * a location those sites could never be mapped at all. Whatever happens here,
 * the operator can still type an address and the coordinate fields beside it.
 */
export function AddressAutocomplete(props: AddressAutocompleteProps) {
  const { key } = useGoogleMapsKey();

  if (!key) {
    return <PlainAddressInput {...props} />;
  }

  return (
    <APIProvider apiKey={key} libraries={["places"]}>
      <SuggestingAddressInput {...props} />
    </APIProvider>
  );
}

function PlainAddressInput({ value, onChange }: AddressAutocompleteProps) {
  return (
    <Input
      value={value ?? ""}
      placeholder="Address"
      onChange={(event) => onChange?.(event.target.value)}
    />
  );
}

function SuggestingAddressInput({
  value,
  onChange,
  onResolved,
}: AddressAutocompleteProps) {
  // Null until the library finishes loading, and it stays null if the script
  // never arrives — which leaves a working text field rather than an error the
  // operator cannot act on mid-form.
  const places = useMapsLibrary("places");
  const inputRef = useRef<InputRef>(null);

  useEffect(() => {
    const element = inputRef.current?.input;
    if (!places || !element) {
      return;
    }

    const autocomplete = new places.Autocomplete(element, {
      fields: ["formatted_address", "geometry"],
    });

    const listener = autocomplete.addListener("place_changed", () => {
      const place = autocomplete.getPlace();
      const location = place.geometry?.location;
      if (!location) {
        return;
      }
      const address = place.formatted_address ?? element.value;
      onChange?.(address);
      onResolved({
        address,
        latitude: location.lat(),
        longitude: location.lng(),
      });
    });

    return () => listener.remove();
  }, [places, onChange, onResolved]);

  return (
    <Input
      ref={inputRef}
      value={value ?? ""}
      placeholder="Start typing an address"
      onChange={(event) => onChange?.(event.target.value)}
    />
  );
}
```

`google.maps.places.Autocomplete` types come from `@types/google.maps`, which
`@vis.gl/react-google-maps` already depends on — no separate install.

**Implementer note:** confirm the current Places autocomplete surface against
live Google documentation before finalising `SuggestingAddressInput` — Google
has moved between `Autocomplete` and `PlaceAutocompleteElement`. Only the body
of that component may change; `AddressAutocomplete`'s props, its plain-input
fallback, and the manual coordinate fields from Task 6 are the contract and must
keep working regardless.

- [ ] **Step 5: Use it in the modal**

In `frontend/src/presentation/components/sites/SiteModal.tsx`, replace the
`location` form item with:

```tsx
        {/* Form.Item clones its child and injects value/onChange, and those
            win over anything passed here — so the field is form-controlled and
            only onResolved is ours to supply. */}
        <Form.Item name="location" label="Address">
          <AddressAutocomplete
            onResolved={(place) => {
              form.setFieldsValue({
                location: place.address,
                latitude: place.latitude.toString(),
                longitude: place.longitude.toString(),
              });
              void form.validateFields(["latitude"]);
            }}
          />
        </Form.Item>
```

and add `import { AddressAutocomplete } from "./AddressAutocomplete";` to its imports.

Add to `frontend/src/presentation/components/sites/index.ts`:

```ts
export * from "./AddressAutocomplete";
export * from "./siteCoordinates";
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd frontend && npm test -- --run src/presentation/components/sites`
Expected: PASS — the autocomplete tests plus the Task 6 modal tests, which must
still pass unchanged since manual entry is untouched.

- [ ] **Step 7: Run the full frontend gate**

Run: `cd frontend && npm run format && npm test -- --run && npm run lint && npm run format:check && npm run build`
Expected: all green.

- [ ] **Step 8: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src
git commit -m "feat(sites): suggest addresses and fill coordinates from Places"
```

---

## Task 8: Map page

**Files:**
- Create: `frontend/src/presentation/components/map/SiteMap.tsx`
- Create: `frontend/src/presentation/components/map/UnmappedSitesPanel.tsx`
- Create: `frontend/src/presentation/components/map/UnmappedSitesPanel.test.tsx`
- Create: `frontend/src/presentation/components/map/index.ts`
- Create: `frontend/src/presentation/pages/MapPage.tsx`
- Create: `frontend/src/presentation/pages/__tests__/MapPage.test.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`
- Modify: `frontend/src/presentation/routes/index.tsx`

**Interfaces:**
- Consumes: `useSites()`, `useOlts()`, `useGoogleMapsKey()`; `Site.latitude`/`.longitude`; `Olt.siteId`, `Olt.status`, `OltStatus.ONLINE`.
- Produces: route `/map`; `mappedSites(sites)` and `unmappedSites(sites)` helpers exported from `SiteMap.tsx`.

- [ ] **Step 1: Write the failing test**

`frontend/src/presentation/components/map/UnmappedSitesPanel.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { UnmappedSitesPanel } from "./UnmappedSitesPanel";

const SITE = {
  id: "s1",
  name: "Gudang",
  location: "Belakang kantor",
  description: "",
  oltCount: 1,
  createdAt: "",
  updatedAt: "",
};

describe("UnmappedSitesPanel", () => {
  it("says nothing is missing when every site has a pin", () => {
    render(<UnmappedSitesPanel sites={[]} />);

    expect(screen.getByText(/every site is on the map/i)).toBeInTheDocument();
  });

  it("names the sites the map cannot show", () => {
    // A map with two pins for three sites reads as complete. Naming the gap is
    // the only way an operator learns a site is missing rather than absent.
    render(<UnmappedSitesPanel sites={[SITE]} />);

    expect(screen.getByText("Gudang")).toBeInTheDocument();
    expect(screen.getByText(/1 site has no coordinates/i)).toBeInTheDocument();
  });

  it("counts more than one correctly", () => {
    render(
      <UnmappedSitesPanel sites={[SITE, { ...SITE, id: "s2", name: "Depok" }]} />,
    );

    expect(screen.getByText(/2 sites have no coordinates/i)).toBeInTheDocument();
  });
});
```

`frontend/src/presentation/pages/__tests__/MapPage.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import MapPage from "../MapPage";

const state = {
  key: undefined as string | undefined,
  sites: [] as unknown[],
};

vi.mock("@/application/hooks", () => ({
  useGoogleMapsKey: () => ({ key: state.key, isLoading: false }),
  useSites: () => ({ data: state.sites, isLoading: false }),
  useOlts: () => ({ data: [], isLoading: false }),
}));

vi.mock("@/presentation/components/map/SiteMap", () => ({
  SiteMap: () => <div data-testid="site-map" />,
}));

describe("MapPage", () => {
  beforeEach(() => {
    state.key = "AIzaSyTESTKEY123";
    state.sites = [];
  });

  it("explains a missing key instead of rendering a broken map", () => {
    state.key = undefined;

    render(<MapPage />);

    expect(screen.getByText(/no google maps api key/i)).toBeInTheDocument();
    expect(screen.queryByTestId("site-map")).not.toBeInTheDocument();
  });

  it("renders the map once a key is configured", () => {
    render(<MapPage />);

    expect(screen.getByTestId("site-map")).toBeInTheDocument();
  });

  it("lists sites with no coordinates beside the map", () => {
    state.sites = [
      {
        id: "s1",
        name: "Gudang",
        location: "Belakang kantor",
        description: "",
        oltCount: 1,
        createdAt: "",
        updatedAt: "",
      },
    ];

    render(<MapPage />);

    expect(screen.getByText("Gudang")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npm test -- --run src/presentation/components/map src/presentation/pages/__tests__/MapPage.test.tsx`
Expected: FAIL — modules not found.

- [ ] **Step 3: Write the panel and the map**

`frontend/src/presentation/components/map/UnmappedSitesPanel.tsx`:

```tsx
import { Link } from "react-router-dom";
import type { Site } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { DarkCard } from "../common";

interface UnmappedSitesPanelProps {
  sites: Site[];
}

/**
 * Sites the map cannot draw. Without this the page quietly lies: two pins for
 * three sites reads as complete, and an operator concludes everything is
 * mapped. An empty result and an unknown result must not look alike.
 */
export function UnmappedSitesPanel({ sites }: UnmappedSitesPanelProps) {
  return (
    <DarkCard title="Not on the map" style={{ height: "100%" }}>
      {sites.length === 0 ? (
        <div style={{ color: colors.textSecondary, fontSize: 13 }}>
          Every site is on the map.
        </div>
      ) : (
        <>
          <div style={{ color: colors.textSecondary, fontSize: 12 }}>
            {sites.length === 1
              ? "1 site has no coordinates"
              : `${sites.length} sites have no coordinates`}
          </div>
          <div style={{ marginTop: 12 }}>
            {sites.map((site, index) => (
              <div
                key={site.id}
                style={{
                  padding: "8px 0",
                  borderTop: index === 0 ? "none" : `1px solid ${colors.border}`,
                }}
              >
                <div style={{ color: colors.textBody, fontSize: 13 }}>
                  {site.name}
                </div>
                <div style={{ color: colors.textMuted, fontSize: 11 }}>
                  {site.location || "no address"}
                </div>
              </div>
            ))}
          </div>
          <div style={{ marginTop: 12, fontSize: 12 }}>
            <Link to="/sites">Add coordinates on the Sites page</Link>
          </div>
        </>
      )}
    </DarkCard>
  );
}
```

`frontend/src/presentation/components/map/SiteMap.tsx`:

```tsx
import { useState } from "react";
import {
  APIProvider,
  AdvancedMarker,
  InfoWindow,
  Map,
} from "@vis.gl/react-google-maps";
import { OltStatus, type Olt, type Site } from "@/domain/entities";

interface SiteMapProps {
  apiKey: string;
  sites: Site[];
  olts: Olt[];
}

// Indonesia, so an installation with no pins yet opens somewhere recognisable
// rather than in the Atlantic at 0,0.
const FALLBACK_CENTER = { lat: -2.5, lng: 118 };
const FALLBACK_ZOOM = 4;
// A single pin should not open at maximum zoom, where there is no context.
const SINGLE_SITE_ZOOM = 14;

/** Sites carrying both coordinates, which are the only ones that can be drawn. */
export function mappedSites(sites: Site[] | undefined): Site[] {
  return (sites ?? []).filter(
    (site) => typeof site.latitude === "number" && typeof site.longitude === "number",
  );
}

/** Sites the map cannot place, which the page must still account for. */
export function unmappedSites(sites: Site[] | undefined): Site[] {
  return (sites ?? []).filter(
    (site) => typeof site.latitude !== "number" || typeof site.longitude !== "number",
  );
}

export function SiteMap({ apiKey, sites, olts }: SiteMapProps) {
  const [selected, setSelected] = useState<Site | null>(null);
  const pins = mappedSites(sites);

  const center =
    pins.length > 0
      ? { lat: pins[0].latitude as number, lng: pins[0].longitude as number }
      : FALLBACK_CENTER;
  const zoom = pins.length > 0 ? SINGLE_SITE_ZOOM : FALLBACK_ZOOM;

  return (
    <APIProvider apiKey={apiKey}>
      <Map
        style={{ width: "100%", height: 520, borderRadius: 8 }}
        defaultCenter={center}
        defaultZoom={zoom}
        mapId="tikman-sites"
        gestureHandling="greedy"
        disableDefaultUI={false}
      >
        {pins.map((site) => (
          <AdvancedMarker
            key={site.id}
            position={{
              lat: site.latitude as number,
              lng: site.longitude as number,
            }}
            title={site.name}
            onClick={() => setSelected(site)}
          />
        ))}

        {selected && (
          <InfoWindow
            position={{
              lat: selected.latitude as number,
              lng: selected.longitude as number,
            }}
            onCloseClick={() => setSelected(null)}
          >
            <SiteSummary site={selected} olts={olts} />
          </InfoWindow>
        )}
      </Map>
    </APIProvider>
  );
}

function SiteSummary({ site, olts }: { site: Site; olts: Olt[] }) {
  const owned = olts.filter((olt) => olt.siteId === site.id);
  const online = owned.filter((olt) => olt.status === OltStatus.ONLINE).length;

  return (
    <div style={{ color: "#18181b", minWidth: 160 }}>
      <div style={{ fontWeight: 600 }}>{site.name}</div>
      <div style={{ fontSize: 12 }}>{site.location || "no address"}</div>
      <div style={{ fontSize: 12, marginTop: 6 }}>
        {owned.length === 0
          ? "No OLTs at this site"
          : `${online} of ${owned.length} OLTs online`}
      </div>
    </div>
  );
}
```

`frontend/src/presentation/components/map/index.ts`:

```ts
export * from "./SiteMap";
export * from "./UnmappedSitesPanel";
```

`frontend/src/presentation/pages/MapPage.tsx`:

```tsx
import { Alert, Col, Row, Skeleton } from "antd";
import { Link } from "react-router-dom";
import { useGoogleMapsKey, useOlts, useSites } from "@/application/hooks";
import { PageHeader, DarkCard } from "../components/common";
// Imported from their own modules rather than the barrel so a test can mock
// the map without also mocking the panel beside it.
import { SiteMap, mappedSites, unmappedSites } from "../components/map/SiteMap";
import { UnmappedSitesPanel } from "../components/map/UnmappedSitesPanel";

export default function MapPage() {
  const { key, isLoading: keyLoading } = useGoogleMapsKey();
  const { data: sites, isLoading: sitesLoading } = useSites();
  const { data: olts } = useOlts();

  const unmapped = unmappedSites(sites);

  return (
    <div>
      <PageHeader
        title="Site Map"
        description={`${mappedSites(sites).length} sites on the map`}
      />

      {keyLoading || sitesLoading ? (
        <Skeleton active paragraph={{ rows: 8 }} title={false} />
      ) : !key ? (
        <Alert
          type="info"
          showIcon
          message="No Google Maps API key is configured"
          description={
            <span>
              The map needs a key before it can draw anything. Add one under{" "}
              <Link to="/settings">Settings</Link>.
            </span>
          }
        />
      ) : (
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={17}>
            <DarkCard style={{ height: "100%" }}>
              <SiteMap apiKey={key} sites={sites ?? []} olts={olts ?? []} />
            </DarkCard>
          </Col>
          <Col xs={24} lg={7}>
            <UnmappedSitesPanel sites={unmapped} />
          </Col>
        </Row>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Add the navigation entry and the route**

In `frontend/src/presentation/components/layout/navigationRoutes.tsx`, add
`GlobalOutlined` to the icon imports and this entry immediately after the
`/sites` entry:

```tsx
    { path: "/map", name: "Map", icon: <GlobalOutlined /> },
```

Add this case to `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`:

```tsx
  it("lists Map for every role, so the page is reachable without its URL", () => {
    // The VPN entry was once added to a component nothing rendered, which left
    // that page invisible to every operator. This file exists to catch that.
    expect(paths(UserRole.ADMIN)).toContain("/map");
    expect(paths(UserRole.VIEWER)).toContain("/map");
  });
```

In `frontend/src/presentation/routes/index.tsx`, add
`import MapPage from "../pages/MapPage";` beside the other page imports and this
route object after the `sites` entry:

```tsx
          {
            path: "map",
            element: <MapPage />,
          },
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd frontend && npm test -- --run src/presentation/components/map src/presentation/pages/__tests__/MapPage.test.tsx`
Expected: PASS, 6 tests.

- [ ] **Step 6: Run the full frontend gate**

Run: `cd frontend && npm run format && npm test -- --run && npm run lint && npm run format:check && npm run build`
Expected: all green.

- [ ] **Step 7: Commit**

```bash
git add frontend/src
git commit -m "feat(map): show sites as pins and name the ones without coordinates"
```

---

## Task 9: Operator documentation

**Files:**
- Modify: `docs/operator_guide.md`

**Interfaces:**
- Consumes: everything above.
- Produces: nothing code depends on.

- [ ] **Step 1: Add a section to the operator guide**

Append to `docs/operator_guide.md`:

```markdown
## Site map

The map draws a pin for every site that has coordinates. Sites without them are
listed beside the map, so a short map is never mistaken for a complete one.

### One-time setup

1. In Google Cloud Console, create a project and enable billing. Places
   autocomplete needs it even inside the free monthly quota.
2. Enable **Maps JavaScript API** and **Places API**.
3. Create an API key.
4. Restrict it: Application restrictions → Websites → add
   `https://noc.radpro.id/*`; API restrictions → Maps JavaScript API and
   Places API only.
5. In TikMan, go to **Settings** and save the key under Google Maps API key.

Step 4 is not optional. The key is delivered to every browser that opens the
map and can be read from the developer tools; the restriction is the only thing
stopping someone else from spending your quota with it.

### Giving a site coordinates

Edit the site and start typing in the **Address** field — suggestions appear
once the key is set, and choosing one fills the coordinates.

For a location Google does not know — a POP down a gang, a tower in a field —
type the latitude and longitude directly. Both must be filled or both left
empty; one on its own is refused, because it would place the pin somewhere the
site is not.
```

- [ ] **Step 2: Commit**

```bash
git add docs/operator_guide.md
git commit -m "docs: how to set up the site map"
```

---

## Deployment

After Task 9, deploy all three containers: the API carries the migrations and
the settings endpoints, the worker shares the API's image, and the frontend
carries the new pages.

```bash
ssh radpro 'cd /opt/tikman/src && git fetch -q origin && git reset --hard -q origin/main && \
  sudo docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml build api worker frontend && \
  sudo docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml up -d api worker frontend'
```

Verify:

```bash
# migrations applied
ssh radpro "sudo docker exec tikman-postgres psql -U tikman -d tikman -t -A -c \
  \"SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 3;\""
# expect 28, 27, 26

# the browser endpoint answers empty before a key is saved, and never 404s
curl -s -o /dev/null -w '%{http_code}\n' https://noc.radpro.id/api/v1/settings/browser
```
