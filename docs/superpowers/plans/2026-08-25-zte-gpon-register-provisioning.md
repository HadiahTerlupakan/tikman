# ZTE GPON Register Provisioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the generic ONT provisioning payload with a ZTE C300/C320 GPON Register flow that registers an ONU and applies one Internet WAN service using VLAN and PPPoE settings.

**Architecture:** The flow is split into identity registration and service configuration. A typed ZTE request is validated, converted into deterministic CLI commands by a pure command builder, executed through the existing per-OLT commander, and tracked by the existing job/snapshot/rollback services. The frontend uses the same typed request to render a GPON Register form and a one-service Internet configuration form; HSGQ and multi-service support remain out of scope.

**Tech Stack:** Go 1.25, Gin, GORM, PostgreSQL/TimescaleDB, existing `OLTCommander`/`CommanderFactory`, React 19, TypeScript, Ant Design, React Query, Vitest, Testing Library.

**Spec:** `docs/superpowers/specs/2026-08-24-provisioning-system-design.md` plus the approved GPON Register reference screenshots supplied by the user on 2026-08-25.

## Global Constraints

- Target vendors: ZTE C300 and ZTE C320 only; reject HSGQ for this flow.
- Do not use placeholder commands such as `serial XXXX`, `profile 1`, or a hard-coded ONT ID.
- All user-supplied fields are validated before any command is sent to the OLT.
- PPPoE passwords are never logged, returned in job responses, or written to audit payloads.
- Provisioning remains asynchronous at the HTTP boundary only if the job ID is returned immediately; command execution and final status must be persisted.
- Existing `JobService`, `SnapshotService`, `RollbackEngine`, and `CommanderFactory` remain the integration boundaries; do not create a second job or rollback system.
- Every production behavior change requires a test written and observed failing before implementation.
- No new dependency is allowed; use existing Go and frontend libraries.
- Keep Go files below the repository’s 300–350 line limit by splitting command generation, validation, handlers, and DTOs.
- Run backend `go test ./... -race`, `go build ./...`, `go vet ./...`, `gofmt -s -l .`, and `golangci-lint run ./...` before completion.
- Run frontend `npm test -- --run`, `npm run lint`, `npm run format:check`, and `npm run build` before completion.

---

## File Map

**Backend new files**
- `backend/internal/models/zte_provisioning.go` — typed request/value objects and constants for ZTE registration and one Internet service.
- `backend/internal/services/zte_provision_validator.go` — pure field validation and ONT ID allocation contract.
- `backend/internal/services/zte_command_builder.go` — pure deterministic ZTE C300/C320 command generation.
- `backend/internal/services/zte_gpon_register_service.go` — orchestration from request to job, snapshots, commands, verification, and rollback.
- `backend/internal/api/zte_provision_dto.go` — HTTP request/response DTOs and conversion helpers.
- `backend/internal/api/zte_provision_handler.go` — register/configure endpoints only.

**Backend modified files**
- `backend/internal/api/router.go` — register ZTE provisioning endpoints.
- `backend/internal/api/olt_handler_discovery.go` — expose the existing unconfigured ONU data as a registration entry point if needed; no direct DB access.
- `backend/internal/services/ont_service.go` — add atomic ONT ID allocation and registration lookup methods.
- `backend/internal/services/job_service.go` — persist the normalized ZTE request in `config_snapshot` and preserve status transitions.
- `backend/internal/services/rollback_engine.go` — add ZTE registration/service restore commands for the new snapshot shape.

**Backend tests**
- `backend/internal/services/zte_command_builder_test.go`
- `backend/internal/services/zte_provision_validator_test.go`
- `backend/internal/services/zte_gpon_register_service_test.go`
- `backend/internal/api/zte_provision_handler_test.go`
- `backend/internal/services/rollback_engine_test.go` — extend existing ZTE coverage.

**Frontend new files**
- `frontend/src/domain/entities/ZteProvisioning.ts` — request types and enums.
- `frontend/src/infrastructure/repositories/ZteProvisioningRepository.ts` — register/configure API calls.
- `frontend/src/application/hooks/useZteProvisioning.ts` — mutations and job polling.
- `frontend/src/presentation/components/zte-provisioning/OnuIdentityForm.tsx` — OLT/card/PON/ONU/serial/type/VEIP fields.
- `frontend/src/presentation/components/zte-provisioning/InternetServiceForm.tsx` — one Internet service WAN/VLAN/PPPoE fields.
- `frontend/src/presentation/components/zte-provisioning/ZteProvisionModal.tsx` — step container, confirmation, command preview, status.
- `frontend/src/presentation/components/zte-provisioning/ZteCommandPreview.tsx` — redacted command preview.
- `frontend/src/presentation/components/zte-provisioning/index.ts` — exports.

**Frontend modified files**
- `frontend/src/infrastructure/http/endpoints.ts` — ZTE registration/configuration endpoints.
- `frontend/src/presentation/pages/UnconfiguredOnusPage.tsx` — add Register action for a discovered ONU.
- `frontend/src/presentation/pages/OntListPage.tsx` — open service configuration for an existing registered ONT.
- `frontend/src/presentation/components/OntTable.tsx` — add Configure Service action without changing existing monitoring actions.
- `frontend/src/domain/entities/index.ts` and `frontend/src/infrastructure/repositories/index.ts` — exports.

---

## Task 1: Define the ZTE request model and validation contract

**Files:**
- Create: `backend/internal/models/zte_provisioning.go`
- Create: `backend/internal/services/zte_provision_validator.go`
- Test: `backend/internal/services/zte_provision_validator_test.go`

**Interfaces:**
- Produces `models.ZTEGPONRegisterRequest` consumed by the handler/service.
- Produces `services.ValidateZTEGPONRegister(req, olt) error`.
- Produces `services.ResolveZTEONUID(ctx, oltID, portID, requestedID) (int, error)`.

- [ ] **Step 1: Write failing validation tests** for these exact cases:
  - C300 and C320 are accepted; HSGQ is rejected.
  - Card, PON, and serial number are required.
  - Custom ONU ID must be 1–127; auto mode accepts zero.
  - Serial number must be 12 uppercase alphanumeric characters after normalization.
  - ONU type must be non-empty and at most 64 characters.
  - `service_enabled` must be true for the first implementation.
  - VLAN ID must be 1–4094.
  - WAN mode must be `pppoe`; service type must be `internet`.
  - PPPoE username/password are required and cannot contain whitespace; password is not included in errors.

- [ ] **Step 2: Run the validator tests and confirm they fail**

```bash
cd backend && go test ./internal/services -run TestValidateZTEGPONRegister -v
```

Expected: compile failure because the request type and validator do not exist.

- [ ] **Step 3: Implement typed models and validation**

```go
type ZTEONUIDMode string
const (
    ZTEONUIDAuto ZTEONUIDMode = "auto"
    ZTEONUIDCustom ZTEONUIDMode = "custom"
)

type ZTEGPONRegisterRequest struct {
    OLTID uuid.UUID `json:"olt_id"`
    Card int `json:"card"`
    PON int `json:"pon"`
    ONUIDMode ZTEONUIDMode `json:"onu_id_mode"`
    ONUID int `json:"onu_id"`
    SerialNumber string `json:"serial_number"`
    ONUType string `json:"onu_type"`
    UseVEIP bool `json:"use_veip"`
    Name string `json:"name"`
    Description string `json:"description"`
    ServiceEnabled bool `json:"service_enabled"`
    VLANMode string `json:"vlan_mode"`
    ServiceType string `json:"service_type"`
    VLANID int `json:"vlan_id"`
    DownloadProfile string `json:"download_profile"`
    UploadProfile string `json:"upload_profile"`
    WANMode string `json:"wan_mode"`
    VLANProfile string `json:"vlan_profile"`
    PPPoEUsername string `json:"pppoe_username"`
    PPPoEPassword string `json:"pppoe_password"`
}
```

`ResolveZTEONUID` must query `onts` for used `(olt_id, port_id, ont_id)` values and return the first free ID in 1–127 for auto mode. It must return a conflict error if a custom ID is already used.

- [ ] **Step 4: Run focused tests and confirm green**

```bash
cd backend && go test ./internal/services -run 'TestValidateZTEGPONRegister|TestResolveZTEONUID' -v
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/zte_provisioning.go backend/internal/services/zte_provision_validator.go backend/internal/services/zte_provision_validator_test.go
 git commit -m "feat(provisioning): define ZTE GPON registration request and validation"
```

---

## Task 2: Build deterministic ZTE C300/C320 command sequences

**Files:**
- Create: `backend/internal/services/zte_command_builder.go`
- Test: `backend/internal/services/zte_command_builder_test.go`

**Interfaces:**
- Consumes `models.ZTEGPONRegisterRequest` and resolved ONU ID.
- Produces `services.BuildZTEGPONRegisterCommands(req, onuID) []string`.
- Produces `services.RedactZTECommands(commands []string) []string` for UI/audit preview.

- [ ] **Step 1: Write failing command fixture tests** asserting exact ordered commands for a ZTE C300 and C320 request:

```go
want := []string{
    "configure terminal",
    "interface gpon-olt_1/3/1",
    "onu 7 type HG8245H5 sn HWTCB403E8A0",
    "exit",
    "interface gpon-onu_1/3/1:7",
    "name 258179206252-Saraswati",
    "tcont 1 name internet profile-name 100M",
    "gemport 1 name internet tcont 1",
    "service-port 1 vport 1 user-vlan 100 vlan 100",
    "wan-ip 1 mode pppoe username example-user password <redacted> vlan-profile INTERNET",
    "exit",
    "commit",
}
```

The test must also assert that a password never appears in redacted output and that invalid service mode does not produce commands.

- [ ] **Step 2: Run command builder tests and confirm red**

```bash
cd backend && go test ./internal/services -run TestBuildZTEGPONRegisterCommands -v
```

- [ ] **Step 3: Implement the builder**

Rules:
- Use `interface gpon-olt_1/<card>/<pon>` for the registration context.
- Use `interface gpon-onu_1/<card>/<pon>:<onu_id>` for service configuration.
- Use the request’s actual ONU type, serial, name, VLAN, profiles, and PPPoE username.
- Never interpolate the raw PPPoE password into preview/log output; the executor receives the real command only at execution time.
- Add `shutdown`/`no shutdown` only when the request explicitly asks for it; the first flow must not invent device state changes.
- End with `commit` exactly once.

- [ ] **Step 4: Run focused builder tests and confirm green**

```bash
cd backend && go test ./internal/services -run 'TestBuildZTEGPONRegisterCommands|TestRedactZTECommands' -v
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/zte_command_builder.go backend/internal/services/zte_command_builder_test.go
 git commit -m "feat(provisioning): build deterministic ZTE GPON register commands"
```

---

## Task 3: Integrate ZTE registration with jobs, snapshots, verification, and rollback

**Files:**
- Create: `backend/internal/services/zte_gpon_register_service.go`
- Modify: `backend/internal/services/job_service.go`
- Modify: `backend/internal/services/rollback_engine.go`
- Test: `backend/internal/services/zte_gpon_register_service_test.go`
- Test: `backend/internal/services/rollback_engine_test.go`

**Interfaces:**
- `NewZTEGPONRegisterService(db, jobs, snapshot, commanderFactory, rollback, logger) *ZTEGPONRegisterService`.
- `RegisterAndConfigure(ctx, req, userID) (*models.ProvisioningJob, error)`.
- `ConfigureExisting(ctx, ontID, req, userID) (*models.ProvisioningJob, error)`.

- [ ] **Step 1: Write failing service tests**:
  - A valid request captures before state, creates pending → running job, sends the exact built commands, and marks success.
  - A commander failure marks failed and invokes rollback with the before snapshot.
  - A duplicate custom ONU ID is rejected before any commander call.
  - An auto ONU ID is allocated and persisted to the registered ONT.
  - `ConfigureExisting` refuses an ONT belonging to a non-ZTE OLT.
  - PPPoE password is absent from logs/audit values and only present in the executor command.

- [ ] **Step 2: Run tests and confirm red**

```bash
cd backend && go test ./internal/services -run TestZTEGPONRegisterService -v
```

- [ ] **Step 3: Implement the orchestration service**

Flow:
1. Load OLT and validate model is C300/C320.
2. Validate request.
3. Resolve auto/custom ONU ID.
4. Capture before snapshot when configuring an existing ONT; for new registration, capture the target position/inventory baseline and persist it as the job’s before snapshot.
5. Persist normalized request in `config_snapshot` with the PPPoE password redacted; keep the runtime command password only in memory.
6. Create the job and transition to `running`.
7. Create the per-OLT commander through `CommanderFactory`.
8. Execute commands sequentially through `BatchExecute`.
9. Read back the ONT position and service identity; require serial and position match.
10. Mark success. On any error, mark failed, invoke `RollbackEngine`, and transition to rolled_back only when restore succeeds.

- [ ] **Step 4: Extend rollback commands**
  - For a new registration, rollback must delete the newly created ONU/service from the exact card/PON/ONU context.
  - For existing service configuration, rollback must restore the before snapshot without deleting the ONU.
  - Add tests asserting idempotent command generation and no password leakage.

- [ ] **Step 5: Run focused and race tests**

```bash
cd backend && go test ./internal/services -run 'TestZTEGPONRegisterService|TestRollbackEngine' -race -v
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/zte_gpon_register_service.go backend/internal/services/job_service.go backend/internal/services/rollback_engine.go backend/internal/services/zte_gpon_register_service_test.go backend/internal/services/rollback_engine_test.go
 git commit -m "feat(provisioning): integrate ZTE GPON registration with jobs and rollback"
```

---

## Task 4: Add protected ZTE registration/configuration API endpoints

**Files:**
- Create: `backend/internal/api/zte_provision_dto.go`
- Create: `backend/internal/api/zte_provision_handler.go`
- Modify: `backend/internal/api/router.go`
- Test: `backend/internal/api/zte_provision_handler_test.go`

**Interfaces:**
- `POST /api/v1/olts/:olt_id/gpon/register`
- `POST /api/v1/onts/:ont_id/gpon/configure`
- `GET /api/v1/provision-jobs/:id`
- Request body is the JSON form of `models.ZTEGPONRegisterRequest`.

- [ ] **Step 1: Write failing handler tests**:
  - Invalid OLT UUID returns 400 `INVALID_ID`.
  - HSGQ model returns 400 `UNSUPPORTED_VENDOR`.
  - Missing serial/card/PON/VLAN/PPPoE fields returns 400 `VALIDATION_ERROR`.
  - `confirm=false` returns 400 `NOT_CONFIRMED`.
  - Authorized technician can submit; viewer receives 403.
  - Successful service call returns 202 with `job_id`, `status`, `onu_id`, and redacted command preview.

- [ ] **Step 2: Run handler tests and confirm red**

```bash
cd backend && go test ./internal/api -run TestZTEProvisionHandler -v
```

- [ ] **Step 3: Implement DTOs, handlers, and route registration**

Use `middleware.AuthMiddleware` on all routes and `RequireRole(admin, technician)` for write operations. Never bind or return PPPoE password in a response preview. Return `202 Accepted` when the job is queued and `200` only for read-only job/status requests.

- [ ] **Step 4: Run API tests and build**

```bash
cd backend && go test ./internal/api -run TestZTEProvisionHandler -v
cd backend && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/api/zte_provision_dto.go backend/internal/api/zte_provision_handler.go backend/internal/api/router.go backend/internal/api/zte_provision_handler_test.go
 git commit -m "feat(api): expose ZTE GPON register and service configuration endpoints"
```

---

## Task 5: Add frontend ZTE GPON Register form

**Files:**
- Create: `frontend/src/domain/entities/ZteProvisioning.ts`
- Create: `frontend/src/infrastructure/repositories/ZteProvisioningRepository.ts`
- Create: `frontend/src/application/hooks/useZteProvisioning.ts`
- Create: `frontend/src/presentation/components/zte-provisioning/OnuIdentityForm.tsx`
- Create: `frontend/src/presentation/components/zte-provisioning/InternetServiceForm.tsx`
- Create: `frontend/src/presentation/components/zte-provisioning/ZteCommandPreview.tsx`
- Create: `frontend/src/presentation/components/zte-provisioning/ZteProvisionModal.tsx`
- Create: `frontend/src/presentation/components/zte-provisioning/index.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Modify: `frontend/src/presentation/pages/UnconfiguredOnusPage.tsx`
- Modify: `frontend/src/presentation/pages/OntListPage.tsx`
- Modify: `frontend/src/presentation/components/OntTable.tsx`
- Test: `frontend/src/presentation/components/zte-provisioning/ZteProvisionModal.test.tsx`

**Interfaces:**
- `ZteGPONRegisterRequest` mirrors the backend JSON keys through humps conversion.
- `useZteGPONRegister().mutate({ oltId, data })` posts registration.
- `useZteExistingService().mutate({ ontId, data })` posts existing ONT configuration.

- [ ] **Step 1: Write failing component tests**:
  - Form renders OLT/card/PON/ONU mode/serial/ONU type/VEIP/name/description fields.
  - Selecting `Auto` hides/disables custom ONU ID; selecting `Custom` enables it.
  - Form renders WAN tabs and one Service 1 with VLAN mode, Internet service type, VLAN ID, download/upload profiles, PPPoE mode/profile/username/password.
  - Submit is disabled until required fields and confirmation are valid.
  - Command preview redacts PPPoE password.
  - Error response is shown without exposing the password.

- [ ] **Step 2: Run component tests and confirm red**

```bash
cd frontend && npm test -- --run src/presentation/components/zte-provisioning/ZteProvisionModal.test.tsx
```

- [ ] **Step 3: Implement typed entities, repository, hooks, and forms**
  - Keep identity and service forms separate so later services do not modify registration fields.
  - Use `Form.List` only if needed for the fixed four service sections; first scope renders Service 1 and collapsed disabled Service 2–4 labels.
  - Render a command preview with password redacted; the preview is informational and never the source of execution.
  - Add a warning that the first implementation targets ZTE C300/C320.

- [ ] **Step 4: Connect entry points**
  - Add `Register` to Unconfigured ONU rows, prefilled with OLT/card/PON/serial data.
  - Add `Configure Service` to existing ONT rows, prefilled with OLT/card/PON/ONU/serial data and without registration-only fields.
  - Preserve existing monitoring, history, and delete actions.

- [ ] **Step 5: Run frontend tests and build**

```bash
cd frontend && npm test -- --run
cd frontend && npm run lint
cd frontend && npm run format:check
cd frontend && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src
 git commit -m "feat(frontend): add ZTE GPON register and Internet service form"
```

---

## Task 6: Integration verification and operator documentation

**Files:**
- Create: `backend/internal/services/zte_gpon_register_integration_test.go`
- Modify: `docs/api_reference.md`
- Modify: `docs/operator_guide.md`
- Modify: `backend/TEST_TRAFFIC_STATS.md` only if command/test setup references provisioning.

- [ ] **Step 1: Write integration tests against a fake commander and fake ZTE read-back**:
  - Successful registration + service 1.
  - Custom ONU ID conflict.
  - Auto ONU ID allocation.
  - CLI command failure → rollback commands.
  - Read-back mismatch → rollback.
  - Duplicate API submissions → one active job per OLT/position.

- [ ] **Step 2: Run integration tests and confirm expected failures before final implementation**

```bash
cd backend && go test ./internal/services -run TestZTEGPONRegisterIntegration -race -v
```

- [ ] **Step 3: Document the exact request/response flow**
  - Add ZTE GPON Register request fields and examples to `docs/api_reference.md`.
  - Document Auto vs Custom ONU ID behavior.
  - Document Service 1 Internet VLAN/PPPoE fields.
  - Document that HSGQ, IPTV, TR069, static WAN, and Services 2–4 are out of scope for this release.

- [ ] **Step 4: Document operator workflow**
  - Add “Register unconfigured ONU” and “Configure existing ONT service” procedures to `docs/operator_guide.md`.
  - Document rollback behavior and the fact that PPPoE passwords are never shown in previews/logs.
  - Document the required ZTE C300/C320 command/read-back verification.

- [ ] **Step 5: Run final verification**

```bash
cd backend && go test ./... -race
cd backend && go build ./...
cd backend && gofmt -s -l .
cd backend && go vet ./...
cd backend && golangci-lint run ./...
cd frontend && npm test -- --run
cd frontend && npm run lint
cd frontend && npm run format:check
cd frontend && npm run build
```

- [ ] **Step 6: Rebuild and smoke-test Docker without deleting volumes**

```bash
docker compose up -d --build api worker frontend
curl http://localhost:8080/health
```

Use Playwright to verify:
- Unconfigured ONU → Register opens with prefilled serial/card/PON.
- Existing ONT → Configure Service opens.
- Required validation blocks incomplete submit.
- Command preview redacts PPPoE password.
- A submitted job shows `pending/running/success` status.

- [ ] **Step 7: Commit documentation and final verification record**

```bash
git add backend/internal/services/zte_gpon_register_integration_test.go docs/api_reference.md docs/operator_guide.md
 git commit -m "test(docs): verify and document ZTE GPON registration workflow"
```

---

## Scope Exclusions for This Plan

- HSGQ command generation and HSGQ UI.
- IPTV, TR069, static WAN, OMCI PPPoE NAT, setup-via-ONT, and Services 2–4 execution.
- Automated discovery of ONU type/profile catalogs from the OLT.
- Destructive production execution against the real OLT without an explicit operator confirmation and a validated staging/device test.

## Completion Criteria

- A ZTE C300/C320 ONU can be registered from an unconfigured ONU with Auto or Custom ONU ID.
- One Internet service can be configured with VLAN ID, download/upload profiles, and PPPoE credentials.
- The exact commands are deterministic, previewable with secrets redacted, and executed only after confirmation.
- A failed command or read-back mismatch creates a failed job and restores the before snapshot.
- Backend and frontend verification commands in Task 6 pass.
- Docker services are rebuilt with `docker compose up -d --build`; no `down -v` is used.
