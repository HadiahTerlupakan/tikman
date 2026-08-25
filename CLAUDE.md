# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TikMan is a web-based ZTE OLT (Optical Line Terminal) provisioning and monitoring system with a Go backend and React TypeScript frontend.

## Development Commands

### Backend (Go)

**Working directory:** `backend/`

**Development with hot reload:**
```bash
cd backend
air
```

**Run API directly:**
```bash
cd backend
go run cmd/api/main.go
```

**Build:**
```bash
cd backend
go build -o api cmd/api/main.go
```

**Tests:**
```bash
cd backend
go test ./... -v                           # All tests
go test ./internal/services/... -v         # Service layer only
go test ./... -cover -coverprofile=coverage.out  # With coverage
```

**Single test:**
```bash
cd backend
go test -v -run TestFunctionName ./internal/services/
```

### Quick Workflow

**Backend entry points:**
- `cmd/api/main.go` — HTTP API, database, Redis, and startup migrations
- `cmd/worker/main.go` — OLT/ONT polling and monitoring events
- `cmd/seed-events/main.go` — development event seeding utility
- `cmd/probe_hsgq/main.go` — device probe utility

**CI workflow:** `.github/workflows/ci.yml`
- Backend: tests, race detector, coverage, `go vet`, `gofmt`, `go mod verify`, and API/worker builds
- Frontend: Vitest, ESLint, Prettier check, and production build
- Security: Trivy filesystem scan

### Tool Versions

- Go 1.25.x
- Node.js 24
- PostgreSQL 15 / TimescaleDB in production Compose
- Redis 7

### Frontend (React + TypeScript)

**Working directory:** `frontend/`

**Development server:**
```bash
cd frontend
npm run dev
```

**Build:**
```bash
cd frontend
npm run build
```

**Tests:**
```bash
cd frontend
npm test -- --run           # Run tests (bare `npm test` is vitest in watch mode)
npm run test:ui       # Tests with UI
```

**Linting & Formatting:**
```bash
cd frontend
npm run lint
npm run format
```

### Git Hooks

Run once per clone (hooks live in `.git/hooks/`, which git does not track):
```bash
./scripts/install-hooks.sh
```

This installs a `pre-commit` hook that runs prettier over staged
`frontend/src/**` files, so the CI formatting gate cannot fail on drift that was
avoidable locally. Set `SKIP_FORMAT_HOOK=1` to bypass it for one commit.

Husky is deliberately not used: it works by setting `core.hooksPath`, which stops
`.git/hooks/` from running and would silently disable the graphify hooks
installed there.

### Infrastructure

**Start development dependencies (PostgreSQL + Redis):**
```bash
docker-compose -f docker-compose.dev.yml up -d
```

**Start full stack (production-like):**
```bash
docker-compose up -d
```

**Health check:**
```bash
curl http://localhost:8080/health
```

**Development configuration:** `docker-compose.dev.yml` exposes PostgreSQL on
host port `5437`, while PostgreSQL uses port `5432` inside the Docker network.
When running the API directly on the host, use `DB_HOST=localhost` and
`DB_PORT=5437`; when running it in a container, use `DB_HOST=postgres` and
`DB_PORT=5432`. Development Redis uses the password `dev-password`.

## Architecture

### Backend Structure (Go)

The backend uses a **layered architecture** with clear separation of concerns:

**`cmd/api/`** - Application entry point. Initializes config, database, Redis, logger, and starts HTTP server.

**`internal/api/`** - HTTP layer
- `router.go` sets up Gin routes with middleware (auth, RBAC, rate limiting, CORS)
- `*_handler.go` files handle HTTP requests/responses
- `dto.go` defines request/response data transfer objects
- Handlers call services, never access database directly

**`internal/services/`** - Business logic layer
- One service per domain entity (UserService, SiteService, OLTService, AuditService, OLTValidatorService)
- Services contain all business rules and validations
- Services directly use GORM to query the database
- Password encryption happens in UserService
- OLT credential encryption happens in OLTService
- OLT creation includes real-time validation (Ping, SSH/Telnet, SNMP connectivity tests)
- OLTValidatorService performs network connectivity validation with strict timeouts (Ping: 2s, SSH/Telnet: 2s, SNMP: 1s)

**`internal/models/`** - GORM models
- Database schema definitions
- **Important:** Foreign key relationship fields (e.g., `Site *Site`, `OLT *OLT`) have been removed to avoid preload issues
- Only foreign key IDs (e.g., `SiteID uuid.UUID`) are kept
- To get related data, query manually using the foreign key ID
- `models.go` defines model metadata; versioned SQL migrations live in `backend/migrations/`

**`backend/migrations/`** - Versioned SQL schema migrations
- Add schema changes as numbered `.up.sql`/`.down.sql` files
- The API applies pending migrations during startup; do not rely on `AutoMigrate` for new schema changes

**`internal/auth/`** - Session management with Redis

**`internal/middleware/`** - HTTP middleware
- `AuthMiddleware` - validates session, sets user in context
- `RequireRole` - RBAC enforcement (Admin, Technician, Viewer)
- Rate limiting

**`internal/config/`** - Configuration via Viper (reads from `.env`)

**`internal/database/`** - GORM database connection setup

**`internal/logger/`** - Structured logging with zap

**`internal/utils/`** - Encryption helpers (AES-256-GCM for OLT credentials)

**`internal/connectivity/`** - Network connectivity testing
- Ping, SSH, Telnet, SNMP connectivity tests
- Used by OLTValidatorService for OLT validation
- All operations have strict timeouts to prevent blocking

### Frontend Structure (React + TypeScript)

The frontend follows **Clean Architecture** principles:

**`src/domain/`** - Core business entities and repository interfaces (pure TypeScript, no framework dependencies)

**`src/application/`** - Application logic
- `stores/` - Zustand stores for global state (auth state)
- `hooks/` - React Query hooks for auth, sites, OLTs, users, ONTs, ONT events, OLT statistics/polling, provisioning, config templates, unconfigured ONUs, and health checks

**`src/infrastructure/`** - External implementations
- `repositories/` - API repository implementations (AuthRepository, SiteRepository, UserRepository, OLTRepository)
- `http/` - Axios client setup with humps for snake_case ↔ camelCase conversion

**`src/presentation/`** - UI components
- `pages/` - Page-level components
- `components/` - Reusable UI components
- Uses Ant Design + Ant Design Pro Components

**`src/shared/`** - Shared utilities
- `config/` - React Query client configuration, environment variables
- `theme/` - Ant Design theme customization

### Key Architectural Patterns

**Backend:**
- No GORM Preload usage - relationships are manual via foreign key IDs
- Services own all business logic, handlers are thin
- Session-based authentication via Redis (HTTP-only cookies)
- Passwords hashed with bcrypt (cost 12)
- OLT credentials encrypted with AES-256-GCM

**Frontend:**
- API responses use snake_case (Go), frontend uses camelCase (humps library handles conversion)
- React Query for server state management
- Zustand for client state (auth)
- Repository pattern abstracts HTTP calls

**Database:**
- PostgreSQL as primary database
- Redis for session storage
- GORM as ORM but without relationship fields to avoid cascade issues

## Common Patterns

### Adding a New API Endpoint

1. Add DTO in `backend/internal/api/dto.go`
2. Add handler method in `backend/internal/api/*_handler.go`
3. Add service method in `backend/internal/services/*_service.go`
4. Register route in `backend/internal/api/router.go` with appropriate middleware
5. Add test in `backend/internal/api/*_handler_test.go`

### Adding a New Model

1. Define struct in `backend/internal/models/`
2. Add to `AutoMigrate()` in `backend/internal/models/models.go`
3. **Do not add** GORM relationship fields (e.g., `Site *Site`) - only IDs
4. Add `BeforeCreate` hook if using UUID primary keys
5. Add `TableName()` method for table name customization

### Frontend Data Fetching

Use React Query hooks from `src/application/hooks/`:
- `useAuth()` - current user, login, logout
- `useSites()` - sites CRUD
- `useOlts()` - OLTs CRUD
- `useUsers()` - users CRUD

## Testing Notes

- Backend tests use in-memory SQLite database
- Tests are isolated - each test gets a fresh database
- Frontend tests use Vitest + Testing Library
- Mock API calls in frontend tests

**CRITICAL: All Tests Must Pass Before Committing**

Before any commit or deployment, ensure ALL tests are green:

### Backend Testing Requirements
```bash
cd backend

# All tests must pass
go test ./... -v

# Run with race detector (CI requirement)
go test ./... -v -race

# Check test coverage (aim for >80%)
go test ./... -cover -coverprofile=coverage.out

# Run linter (local gate; CI runs `go vet`, `gofmt -s`, and `go mod verify` instead)
golangci-lint run
```

### Frontend Testing Requirements
```bash
cd frontend

# All tests must pass (bare `npm test` is vitest in watch mode)
npm test -- --run

# Build must succeed (catches TypeScript errors)
npm run build

# Linting must pass (zero errors)
npm run lint

# Code formatting check (same gate as CI; `npm run format` rewrites files)
npm run format:check
```

### Pre-Commit Checklist
- [ ] All backend tests pass (`go test ./...`)
- [ ] All frontend tests pass (`npm test -- --run`)
- [ ] No linting errors (`npm run lint`, `golangci-lint run` — local-only; CI runs `go vet`, `gofmt -s`, `go mod verify` instead)
- [ ] Code properly formatted (`npm run format:check`)
- [ ] Go code is gofmt-clean and modules verified (`gofmt -s -l .` empty, `go mod verify`)
- [ ] Build succeeds (`npm run build`, `go build`)
- [ ] No race conditions (`go test -race`)
- [ ] Test coverage maintained (≥50% globally; network-bound/auth/cmd packages excluded from baseline)

### CI/CD Testing
All jobs in `.github/workflows/ci.yml` MUST pass:
- Backend tests, race detector, coverage, lint checks, and API/worker builds
- Frontend tests, lint, format check, and build
- Trivy security scan

`.github/workflows/code-quality.yml` is deprecated and only redirects to the
main CI workflow.

**If CI fails, fix immediately before merging.**

## Code Quality Standards

**IMPORTANT: Prevent Code Smell and God Objects**

### File Size Limits
- **Maximum lines per file: 300-350 lines**
- Files exceeding 350 lines must be refactored into smaller modules
- Test files should be split by functionality (e.g., `handler_create_test.go`, `handler_update_test.go`)

### Anti-Patterns to Avoid
1. **God Objects** - Classes/structs with too many responsibilities
   - Split into focused, single-responsibility modules
   - Example: Split large handlers into separate files by operation (CRUD)

2. **Code Duplication**
   - Extract common test setup into helper functions
   - Use table-driven tests for similar test cases
   - Create reusable test fixtures

3. **Long Functions**
   - Maximum 50 lines per function
   - Extract helper functions for complex logic
   - Use meaningful function names that describe intent

4. **Deep Nesting**
   - Maximum 3 levels of indentation
   - Use early returns to reduce nesting
   - Extract nested logic into separate functions

5. **Magic Numbers/Strings**
   - Define constants for all magic values
   - Use enums/constants instead of raw strings

**Exemption: network-bound SNMP/Telnet code.** Functions that construct
`&gosnmp.GoSNMP{}` or dial directly (`snmp_client.go`, `snmp_walks.go`,
`snmp_metrics_walk.go`, `telnet.go`) are exempt from the line limits. They
cannot be unit tested without an interface refactor, a fake SNMP responder, or
real hardware, so splitting them would be verified by `go build` and review
alone. Restructuring untestable code to satisfy a line count is risk without
payoff. Code that only *calls* `connectivity` is not exempt.

**Exemption: test files.** Test files may exceed 350 lines when the excess is
individual test cases (one test function per behaviour, table-driven where
applicable). Split a test file only when it covers multiple behaviours that
belong in separate files (e.g., `handler_create_test.go` vs
`handler_update_test.go`); do not split merely to satisfy the line count. Test
volume is a consequence of coverage, not a code smell.

**Exemption: entry points.** `main()` functions in `cmd/` are exempt from the
50-line function limit. Wiring (config, logger, DB, HTTP server) is linear
setup; forcing it into helpers scatters initialization order and makes
startup harder to follow. Extract helpers only for reusable logic, not to
game the line count.

### Coverage Targets

- **Global: ≥50%** of statements, measured with `go test ./... -coverprofile`
- **Excluded from the baseline** (network-bound or infrastructure-dependent):
  `internal/connectivity` (SNMP/Telnet devices), `internal/auth` (Redis
  sessions), `cmd/*` (entry-point wiring)
- The excluded packages are measured separately and reported, not silently
  dropped: connectivity and auth deserve integration tests against real
  Redis/devices, which is future investment beyond this compliance work
- Do not pad coverage with trivial getter/setter tests to hit the number; a
  test that asserts nothing is worse than an uncovered line

### Refactoring Guidelines
When a file exceeds 300 lines:
- **Test files**: Split by test type (create, update, delete, list, errors)
- **Handlers**: Split by HTTP method or resource operation
- **Services**: Split by business domain or use case
- **Components**: Split by feature or extract sub-components

### Code Review Checklist
Before committing, verify:
- [ ] No file exceeds 350 lines
- [ ] No function exceeds 50 lines
- [ ] No code duplication
- [ ] All magic values are constants
- [ ] Maximum 3 levels of nesting
- [ ] Single responsibility per file/function
- [ ] All tests pass (backend + frontend)
- [ ] No linting errors
- [ ] Code properly formatted
- [ ] Build succeeds without errors
- [ ] No race conditions detected

## AI-Generated Code Standards

**IMPORTANT: No AI Slop. Solve the problem that was asked, nothing more.**

These rules exist because AI-generated code tends to pass every check above while still
being bloated, over-abstracted, and full of noise. Volume is not value.

### Scope Discipline
- Implement **only** what was requested. No extra features, endpoints, config options,
  or "while I was in there" changes.
- A bug fix fixes the bug. Do **not** refactor surrounding code, rename variables, or
  reformat untouched lines in the same change.
- No speculative abstraction. Do not add interfaces, wrappers, factories, or generic
  helpers for a single caller "in case we need it later". Add them when the second
  caller actually exists.
- No defensive code for impossible states. Validate real inputs (user input, network,
  DB); do not nil-check values the compiler already guarantees.

### Comments
- Comments explain **why**, never **what**. If the code needs a comment to say what it
  does, rename it instead.
- Banned: restating the line below (`// increment counter`), section banners
  (`// ===== HANDLERS =====`), narration of obvious steps (`// Step 1: get the user`).
- Keep: non-obvious constraints, ZTE OLT quirks, timeout rationale, links to issues,
  and explanations of why a simpler approach does not work.
- Go doc comments on exported identifiers are required and are not "noise" - keep them
  short and factual.

### No Leftovers
- No dead code, commented-out blocks, unused imports/variables, or unreachable branches.
- No `TODO`, `FIXME`, or placeholder stubs left behind. Either implement it, or state it
  in the PR description as explicitly out of scope.
- No debug output shipped: no stray `fmt.Println`, `console.log`, or `.only()` /
  `t.Skip()` left in tests.
- Delete replaced code. Do not keep the old version alongside the new one.

### Follow What Exists
- Use the libraries, patterns, and naming already in the repo. Do not introduce a new
  dependency, HTTP client, state manager, or test helper when an existing one works.
- Match the surrounding file's style before applying any general "best practice".
- New dependencies require explicit approval and a pinned version.

### Tests, Not Test Theatre
- Tests must assert real behaviour. No tests that only check a mock was called, assert
  `err == nil` and nothing else, or restate the implementation.
- Do not pad coverage with trivial getter/setter tests to hit the 80% number.
- Every bug fix gets a test that fails before the fix and passes after.

### Documentation & Reporting
- Do **not** create new `.md` files (README, SUMMARY, CHANGELOG, MIGRATION_NOTES, etc.)
  unless explicitly requested. Update existing docs instead.
- Do not generate progress-report or hand-off documents as a side effect of finishing work.
- Keep completion summaries to a few sentences: what changed, what was verified, what
  is still open. No emoji headers, no celebration, no restating the whole diff.

### AI Slop Review Checklist
Before committing, verify:
- [ ] Nothing added beyond the stated scope
- [ ] No speculative abstractions or single-use indirection
- [ ] Every comment explains why, not what
- [ ] No dead code, commented-out blocks, or leftover `TODO`s
- [ ] No debug logging or skipped/focused tests
- [ ] No new dependencies or patterns without a reason
- [ ] Tests assert behaviour, not mock bookkeeping
- [ ] No unrequested markdown files created

## Default Credentials

- Username: `admin`
- Password: `admin123`

## Security Notes

- Never commit `.env` file
- OLT credentials are encrypted at rest
- Session tokens are HTTP-only cookies
- All protected routes require authentication
- RBAC enforced via middleware

## Environment Variables

See `.env.example` for required variables. Key ones:
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` - PostgreSQL connection
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` - Redis connection
- `ENCRYPTION_KEY` - 32-byte key for OLT credential encryption
- `SESSION_SECRET` - Secret for session signing

## CI/CD Pipeline

### GitHub Actions Workflows

**CI/CD Pipeline** (`.github/workflows/ci.yml`)
- Runs on push/PR to `main` or `develop`
- Detects backend/frontend changes before running relevant jobs
- Backend tests use the race detector and coverage; lint checks use `go vet`, `gofmt -s`, and `go mod verify`
- Frontend runs Vitest, ESLint, Prettier format checks, and a production build
- Security scanning uses Trivy and uploads results to GitHub Security
- Builds and pushes backend/worker images to GitHub Container Registry on pushes to `main`

**Code Quality** (`.github/workflows/code-quality.yml`)
- Deprecated redirect workflow; security scanning now runs in `ci.yml`

**Deploy** (`.github/workflows/deploy.yml`)
- Runs for published releases or manual production/staging dispatch
- Pulls GHCR images, deploys via SSH, and checks the configured health endpoint


### Deployment Secrets Required

Configure these in GitHub repository settings → Secrets:
- `DEPLOY_HOST` - Server hostname/IP
- `DEPLOY_USER` - SSH username
- `DEPLOY_SSH_KEY` - Private SSH key
- `DEPLOY_PORT` - SSH port (default: 22)
- `DEPLOY_URL` - Application URL for health check

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
