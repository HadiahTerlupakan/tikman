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
npm test              # Run tests
npm run test:ui       # Tests with UI
```

**Linting & Formatting:**
```bash
cd frontend
npm run lint
npm run format
```

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
- `models.go` defines AutoMigrate - currently only migrates `User`, `Site`, `OLT`

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
- `hooks/` - React Query hooks for data fetching (useAuth, useSites, useOlts, useUsers)

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

# Run linter (must have zero issues)
golangci-lint run
```

### Frontend Testing Requirements
```bash
cd frontend

# All tests must pass
npm test

# Build must succeed (catches TypeScript errors)
npm run build

# Linting must pass (zero errors)
npm run lint

# Code formatting check (same gate as CI; `npm run format` rewrites files)
npm run format:check
```

### Pre-Commit Checklist
- [ ] All backend tests pass (`go test ./...`)
- [ ] All frontend tests pass (`npm test`)
- [ ] No linting errors (`npm run lint`, `golangci-lint run`)
- [ ] Code properly formatted (`npm run format:check`)
- [ ] Build succeeds (`npm run build`, `go build`)
- [ ] No race conditions (`go test -race`)
- [ ] Test coverage maintained (>80%)

### CI/CD Testing
All GitHub Actions workflows MUST pass:
- Backend CI (tests + lint + build)
- Frontend CI (tests + lint + format + build)
- Code Quality (security scan)

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
- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string  
- `ENCRYPTION_KEY` - 32-byte key for OLT credential encryption
- `SESSION_SECRET` - Secret for session signing

## CI/CD Pipeline

### GitHub Actions Workflows

**Backend CI** (`.github/workflows/backend-ci.yml`)
- Runs on push/PR to `main` or `develop`
- Tests with race detector and coverage
- Lints with golangci-lint
- Builds binaries and uploads artifacts

**Frontend CI** (`.github/workflows/frontend-ci.yml`)
- Runs on push/PR to `main` or `develop`
- Tests with Vitest and coverage
- Lints with ESLint
- Checks code formatting with Prettier
- Builds production bundle

**Docker Build** (`.github/workflows/docker-build.yml`)
- Builds Docker images for backend and worker
- Pushes to GitHub Container Registry (ghcr.io)
- Tags: branch name, semver, SHA
- Triggered on push to `main` or version tags

**Code Quality** (`.github/workflows/code-quality.yml`)
- Security scanning with Trivy
- Dependency review for PRs
- Go security scanning with Gosec
- Results uploaded to GitHub Security tab

**Deploy** (`.github/workflows/deploy.yml`)
- Manual dispatch or triggered on release
- Deploys to production/staging via SSH
- Pulls latest Docker images
- Runs health check post-deployment

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
