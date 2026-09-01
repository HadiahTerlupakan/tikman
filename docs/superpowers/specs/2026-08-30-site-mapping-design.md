# Site Mapping Design

**Date:** 2026-08-30
**Status:** Approved for planning

## Goal

Let an operator see every OLT site as a pin on a map, and capture the
coordinates that make those pins possible while creating or editing a site.

This needs somewhere to keep a Google Maps API key, so it also introduces a
general settings store that later integrations can reuse.

## Scope

Three parts, built in order. Each is independently useful and independently
testable; part 3 is meaningless without part 2, and part 2 without part 1.

1. **Settings** — an encrypted, named store for API credentials, with an
   admin-only management page.
2. **Site coordinates** — latitude and longitude on `sites`, filled by address
   autocomplete or entered by hand.
3. **Map** — a page showing one pin per mapped site.

## Non-goals

- Geocoding existing sites in bulk. The three current rows keep their free-text
  location until someone edits them.
- Routing, distance, or coverage overlays.
- Per-user or per-site API keys. One key serves the installation.
- Storing anything but credentials in the settings store. Feature toggles and
  user preferences are a different problem with different access rules.

## Current state

`sites` has `id`, `name`, `location` (free text), `description`, and timestamps.
Production holds three rows whose `location` is a city name — `Depok`,
`Bekasi` — so there is nothing to plot today.

There is no settings or configuration table anywhere in the backend.

`internal/utils/crypto.go` already provides `Encrypt(plaintext, key string)` and
`Decrypt(ciphertext, key string)` using AES-256-GCM, keyed by the
`ENCRYPTION_KEY` environment variable. OLT credentials use it. Settings reuse
it; no new crypto is written.

RBAC exists as `middleware.RequireRole(models.UserRoleAdmin, ...)`, applied
per-route in `internal/api/router.go`. `AuditService.Log` records changes.

## Global constraints

- Go 1.25.x, Node.js 24, PostgreSQL 15, existing dependency set plus the one
  addition named below.
- Schema changes ship as numbered SQL under `backend/migrations/`, and the model
  is registered in `models.AutoMigrate`.
- No file over 350 lines; no function over 50 lines.
- Backend: `gofmt -s` clean, `go vet` clean, race detector clean.
- Frontend: Vitest, ESLint, Prettier, and `tsc` all clean.
- UI text on these pages is English, matching Dashboard, Sites, and Graphs.

### New dependency

`@vis.gl/react-google-maps`, pinned at **1.9.0**. Google's own React wrapper for
the Maps JavaScript API. Approved by the user on 2026-08-30.

It is taken instead of hand-rolling script injection because loading and tearing
down the Maps script inside React is where that integration usually breaks —
double-mounting under StrictMode, listeners surviving unmount, the script
loading twice on a fast route change. The wrapper owns that lifecycle.

---

## Security: what the key is and is not

**A Google Maps browser key cannot be kept secret.** Rendering a map and running
address autocomplete both happen in the browser, so the key must reach it. Any
viewer can read it from the network tab. This is how the product works and is
documented by Google.

What actually protects the key is a **restriction set in Google Cloud Console**:
the key must accept requests only from `https://your-noc-domain/*` and only for
the APIs it needs. Without that restriction, a copied key is billable to the
account that owns it.

Encrypting the value at rest is therefore still worth doing, but for a narrower
reason that must not be overstated:

- it protects the key in a database dump or a backup,
- it keeps the key out of reach of any TikMan user who is not an admin,
- it does **not** hide the key from someone who loads the map page.

The settings page states this beside the Maps key, with the restriction steps,
because an operator who believes the key is secret will not go and restrict it.

---

## Part 1 — Settings

### Storage

Table `app_settings`, created by `backend/migrations/27_add_app_settings.sql`
and registered in `models.AutoMigrate`:

| Column | Type | Notes |
|---|---|---|
| `name` | `VARCHAR(64)` | primary key, matches a known setting |
| `value` | `TEXT NOT NULL` | AES-256-GCM ciphertext, never plaintext |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | |
| `updated_by` | `UUID` | the admin who last wrote it; no foreign key, following the repo's practice of storing ids without relationship fields |

Model at `backend/internal/models/app_setting.go`, `TableName()` returning
`app_settings`.

### The registry, and why storage alone is not enough

Storage is generic so a new integration needs no migration. Access is not.

A single "read any setting" endpoint available to every logged-in user would let
a Viewer read every API credential the installation has ever stored. So the code
carries a registry of **known settings**, each declaring who may read it:

```go
type SettingVisibility string

const (
    // Server-only: the value is decrypted for backend use and never leaves it.
    VisibilityServerOnly SettingVisibility = "server_only"
    // Browser: the value is delivered to any authenticated user, because the
    // feature it drives runs in their browser and cannot work otherwise.
    VisibilityBrowser SettingVisibility = "browser"
)

type SettingDefinition struct {
    Name        string
    Label       string
    Description string
    Visibility  SettingVisibility
}
```

Initial registry:

| Name | Label | Visibility | Why |
|---|---|---|---|
| `google_maps_api_key` | Google Maps API key | `browser` | the map and autocomplete run client-side |

**The zero value is `server_only`.** A definition added without stating its
visibility does not reach the browser. Exposing a credential must be a decision
someone typed, not something that happens by forgetting.

Writing or reading a name absent from the registry is rejected. The store is not
an arbitrary key-value bag an attacker can fill.

### Service

`backend/internal/services/setting_service.go`:

- `NewSettingService(db *gorm.DB, encryptionKey string) *SettingService` —
  matching `NewOLTService`'s shape.
- `List() ([]SettingStatus, error)` — one entry per **registry** definition,
  whether stored or not. Carries `Name`, `Label`, `Description`, `Configured`,
  `Preview`, `UpdatedAt`. Never the value.
- `Set(name, value string, actor uuid.UUID) error` — validates the name against
  the registry, rejects an empty or whitespace-only value, encrypts, upserts.
- `Delete(name string) error`.
- `Value(name string) (string, error)` — decrypts for backend callers. Returns
  `ErrSettingNotConfigured` when absent, so a caller can degrade rather than
  guess.
- `BrowserValues() (map[string]string, error)` — decrypts only definitions whose
  visibility is `browser`. This is the only path by which a value leaves the
  server.

`Preview` masks: the first 4 and last 4 characters around exactly 8 dots, e.g.
`AIza••••••••3f2k`. A value shorter than 12 characters renders as those same 8
dots and nothing else — showing most of a short secret defeats the masking, and
a variable dot count would leak the value's length.

### API

All under the authenticated router group.

| Method | Path | Role | Returns |
|---|---|---|---|
| `GET` | `/api/v1/settings` | Admin | every registry entry with its status; no values |
| `PUT` | `/api/v1/settings/:name` | Admin | `{"value": "..."}` in, status out |
| `DELETE` | `/api/v1/settings/:name` | Admin | 204 |
| `GET` | `/api/v1/settings/browser` | any authenticated | `{ "google_maps_api_key": "..." }` — only `browser` definitions |

Registration note: `GET /settings/browser` is a static path while
`PUT`/`DELETE /settings/:name` are parameterised. Gin keeps a separate tree per
method, so these coexist — but a `GET /settings/:name` must not be added later
without moving the browser route out of that path, or the router panics on a
wildcard conflict.

`GET /api/v1/settings/browser` returns an empty object when nothing is
configured, never a 404: "no key is set" is a normal state the frontend handles,
not an error.

An unknown `:name` answers 404 with code `UNKNOWN_SETTING`. A blank value
answers 400 with code `INVALID_VALUE`.

### Audit

`Set` and `Delete` write an `audit_logs` entry through the existing
`AuditService.Log`, with `resourceType` `"setting"`. Settings are keyed by name
rather than by UUID, so `resourceID` is `uuid.Nil` and the name travels in the
new-value map as `{"name": "google_maps_api_key"}`. **The value itself is never logged** — not in the audit row, not in
application logs. An audit trail that records the credential defeats the
encryption it is auditing.

### Settings page

Route `/settings`, sidebar entry **Settings**, visible and reachable by Admin
only. Added to `navigationRoutes.tsx` — the file the layout actually renders.

For each registry entry: label, description, whether it is configured, the
masked preview, when it was last changed. Buttons to set or replace the value,
and to remove it.

Beside `google_maps_api_key`, a permanent panel — not a dismissible hint —
carrying the restriction steps:

1. Google Cloud Console → APIs & Services → Credentials
2. Open the key → Application restrictions → **Websites**
3. Add `https://your-noc-domain/*`
4. API restrictions → restrict to **Maps JavaScript API** and **Places API**

with the plain statement that an unrestricted key is billable by anyone who
copies it out of the page.

---

## Part 2 — Site coordinates

### Schema

`backend/migrations/28_add_site_coordinates.sql` adds to `sites`:

| Column | Type | Notes |
|---|---|---|
| `latitude` | `DOUBLE PRECISION` | nullable |
| `longitude` | `DOUBLE PRECISION` | nullable |

**Nullable is deliberate.** Existing sites have no coordinates, and a site
without them stays valid — not every site needs to be on the map, and a site
must never become unsavable because a location could not be resolved.

`location` keeps its meaning and is reused as the address text. Existing values
survive untouched. A separate "address" column alongside "location" would leave
two fields meaning the same thing and no rule for which wins.

### Validation

In `SiteService`, applied to create and update:

- `latitude` within −90..90, `longitude` within −180..180.
- **Both present or both absent.** A half-set coordinate is not a partial
  answer, it is a bug: it would place a pin on the equator or the prime
  meridian and look deliberate.

Violations return the `ErrValidation` sentinel so the HTTP layer answers 400
with the actionable sentence, matching how WireGuard validation already reports.

### Site form

`SiteModal` gains:

- an **address** field bound to Google Places autocomplete when a Maps key is
  configured, writing both the address text and the resolved coordinates;
- **latitude** and **longitude** fields, always visible and always editable;
- a line showing the current coordinates, or "not mapped" when absent.

Three behaviours are required, and the second and third are not optional
niceties:

**Without a key**, the address field is a plain text input. The page must not
break, warn repeatedly, or block saving because a credential is missing.

**Manual entry always works.** Places autocomplete does not know a POP down a
gang, a tower in a field, or the customer house that happens to be an
aggregation point — which is a large share of the sites this system exists to
manage. If Google were the only way to set coordinates, those sites could never
be mapped at all.

**Autocomplete failure is not save failure.** If the Places request errors or
the script never loads, the operator can still type an address and coordinates
and save the site.

Implementation note: the Maps script is loaded with the `places` library and the
autocomplete uses the current Places autocomplete element
(`google.maps.places.PlaceAutocompleteElement`). Google has moved this surface
before; the implementer should confirm the current API against live
documentation, and the manual path above is the contract that holds regardless.

---

## Part 3 — Map

Route `/map`, sidebar entry **Map**, available to every authenticated role —
seeing where sites are is not a privileged action.

### Content

- A Google map, initially framed to fit every mapped site. With one site, a
  fixed sensible zoom rather than the maximum.
- One pin per site holding coordinates.
- Clicking a pin opens: site name, address, the number of OLTs at that site, and
  how many of them are online. `OLT` already carries `siteId`, so this is a
  grouping of data the app already fetches — no new endpoint.

### Sites that are not mapped

Listed in a panel beside the map, each with a link to edit it.

Without this the page quietly lies: a site with no coordinates simply is not
drawn, and a map showing two pins for three sites reads as complete. Naming the
gap is the same rule applied to the unconfigured-ONU list and the tunnel card —
an empty result and an unknown result must not look alike.

### Missing key

When no Maps key is configured, the page explains that and links to Settings.
It does not render a grey box, a console error, or a partially initialised map.

---

## Testing

**Backend**

- Registry: an unknown name is rejected for read and write; a definition without
  an explicit visibility is `server_only`.
- Service: encrypt/decrypt round-trip; `Value` reports not-configured distinctly
  from an error; `BrowserValues` excludes `server_only` definitions even when
  they are stored.
- Masking: long value shows head and tail; short value shows only dots.
- Handlers: non-admin is refused `GET/PUT/DELETE /settings`; any authenticated
  role may call `/settings/browser`; `/settings/browser` never contains a
  `server_only` name.
- Audit: a write produces an audit row, and the row does not contain the value.
- Site validation: range bounds; both-or-neither; a site with no coordinates
  saves successfully.

**Frontend**

- Settings page: masked display, save, delete, admin-only rendering.
- Site form: works with no key configured; manual coordinate entry saves;
  invalid coordinates are refused with the reason shown.
- Map page: pins for mapped sites; unmapped sites listed and not silently
  dropped; the missing-key state renders its explanation.

Every test asserts behaviour. No test asserts that a mock was called.

## Operational prerequisites

Outside the code, and the feature is inert without them:

1. A Google Cloud project with billing enabled — Places Autocomplete requires
   it, free monthly quota notwithstanding.
2. Maps JavaScript API and Places API enabled on that project.
3. An API key created and restricted as described above.
4. The key saved in TikMan under Settings.

## Roll-out order

Part 1 ships and is usable alone. Part 2 ships next and is usable without a key
through manual entry. Part 3 ships last and needs both.
