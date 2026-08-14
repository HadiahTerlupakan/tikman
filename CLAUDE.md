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
- One service per domain entity (UserService, SiteService, OLTService, AuditService)
- Services contain all business rules and validations
- Services directly use GORM to query the database
- Password encryption happens in UserService
- OLT credential encryption happens in OLTService

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
