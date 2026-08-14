# Source of Truth (SOT) - TikMan

**Last Updated:** 2026-08-15

This document serves as the **single source of truth** for TikMan's architecture, design decisions, and module inventory. All architectural decisions and module changes must be reflected here.

## Quick Reference

| Aspect | Location | Status |
|--------|----------|--------|
| Development Guide | CLAUDE.md | Active |
| API Documentation | backend/docs/API.md | Active |
| Architecture Diagram | docs/architecture.png | Planned |
| Database Schema | backend/docs/DATABASE.md | Planned |

## System Overview

**TikMan** is a web-based ZTE OLT (Optical Line Terminal) provisioning and monitoring system.

**Tech Stack:**
- Backend: Go 1.23+ with Gin framework
- Frontend: React 18+ with TypeScript
- Database: PostgreSQL 15+
- Cache/Session: Redis 7+
- Deployment: Docker + Docker Compose

**Core Features:**
1. Multi-site OLT management
2. User authentication & RBAC (Admin, Technician, Viewer)
3. Audit logging for all operations
4. Encrypted credential storage
5. Session-based authentication

## Architecture Principles

1. **Layered Architecture** - Clear separation: HTTP → Service → Database
2. **No GORM Preload** - Manual relationship queries to avoid cascade issues
3. **Service-Owned Logic** - All business rules in service layer, thin handlers
4. **Clean Architecture (Frontend)** - Domain → Application → Infrastructure → Presentation
5. **Encryption at Rest** - All sensitive data encrypted (passwords, OLT credentials)

## Module Inventory

### Backend Modules

#### `cmd/api/`
**Purpose:** Application entry point  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Initialize application (config, database, Redis, logger)
- Start HTTP server
- Handle graceful shutdown

**Key Files:**
- `main.go` - Entry point

**Dependencies:**
- internal/config
- internal/database
- internal/logger
- internal/api

**Documentation:** [cmd/api/README.md](backend/cmd/api/README.md)

---

#### `internal/api/`
**Purpose:** HTTP layer - handles requests/responses  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Route registration and middleware setup
- HTTP request/response handling
- DTO validation
- Call service layer (never access database directly)

**Key Files:**
- `router.go` - Route setup with middleware
- `*_handler.go` - HTTP handlers per domain
- `dto.go` - Request/response DTOs

**Dependencies:**
- internal/services
- internal/middleware
- internal/models (DTOs only)

**Documentation:** [internal/api/README.md](backend/internal/api/README.md)

---

#### `internal/services/`
**Purpose:** Business logic layer  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- All business rules and validations
- Direct database access via GORM
- Password/credential encryption
- Transaction management

**Key Files:**
- `user_service.go` - User CRUD, authentication
- `site_service.go` - Site management
- `olt_service.go` - OLT CRUD with credential encryption
- `audit_service.go` - Audit log creation

**Design Decisions:**
- Services own all business logic
- No GORM Preload - manual relationship queries
- Each service is self-contained

**Dependencies:**
- internal/models
- internal/utils (encryption)
- GORM

**Documentation:** [internal/services/README.md](backend/internal/services/README.md)

---

#### `internal/models/`
**Purpose:** Database schema definitions  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- GORM model definitions
- Table naming conventions
- UUID primary key hooks
- Auto-migration list

**Key Files:**
- `user.go` - User model
- `site.go` - Site model
- `olt.go` - OLT model
- `audit.go` - Audit log model
- `models.go` - AutoMigrate function

**Design Decisions:**
- **No GORM relationship fields** (e.g., `Site *Site`) - only foreign key IDs
- Manual relationship queries to avoid cascade issues
- UUIDs as primary keys

**Dependencies:**
- GORM

**Documentation:** [internal/models/README.md](backend/internal/models/README.md)

---

#### `internal/middleware/`
**Purpose:** HTTP middleware  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Authentication validation
- RBAC enforcement
- Rate limiting
- CORS handling

**Key Files:**
- `auth.go` - AuthMiddleware, RequireRole
- `rate_limit.go` - Rate limiting
- `cors.go` - CORS configuration

**Dependencies:**
- internal/auth
- internal/models

**Documentation:** [internal/middleware/README.md](backend/internal/middleware/README.md)

---

#### `internal/auth/`
**Purpose:** Session management with Redis  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Session creation/validation/deletion
- Redis session storage
- HTTP-only cookie management

**Key Files:**
- `session.go` - Session manager

**Dependencies:**
- Redis client

**Documentation:** [internal/auth/README.md](backend/internal/auth/README.md)

---

#### `internal/config/`
**Purpose:** Configuration management  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Load configuration from .env via Viper
- Provide config struct to application

**Key Files:**
- `config.go` - Config struct and loader

**Dependencies:**
- Viper

**Documentation:** [internal/config/README.md](backend/internal/config/README.md)

---

#### `internal/database/`
**Purpose:** Database connection setup  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- GORM connection initialization
- Connection pooling configuration

**Key Files:**
- `database.go` - Connection setup

**Dependencies:**
- GORM
- PostgreSQL driver

**Documentation:** [internal/database/README.md](backend/internal/database/README.md)

---

#### `internal/logger/`
**Purpose:** Structured logging  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Zap logger initialization
- Log level configuration

**Key Files:**
- `logger.go` - Logger setup

**Dependencies:**
- Zap

**Documentation:** [internal/logger/README.md](backend/internal/logger/README.md)

---

#### `internal/utils/`
**Purpose:** Shared utilities  
**Owner:** Backend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- AES-256-GCM encryption/decryption for OLT credentials
- Other utility functions

**Key Files:**
- `encryption.go` - Credential encryption

**Dependencies:**
- crypto/aes, crypto/cipher

**Documentation:** [internal/utils/README.md](backend/internal/utils/README.md)

---

### Frontend Modules

#### `src/domain/`
**Purpose:** Core business entities and interfaces  
**Owner:** Frontend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Entity type definitions
- Repository interface contracts
- Pure TypeScript, no framework dependencies

**Key Files:**
- `entities/` - User, Site, OLT, Audit types
- `repositories/` - Repository interfaces

**Dependencies:** None (pure TypeScript)

**Documentation:** [src/domain/README.md](frontend/src/domain/README.md)

---

#### `src/application/`
**Purpose:** Application logic and state management  
**Owner:** Frontend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Global state management (Zustand)
- Data fetching hooks (React Query)
- Application-level business logic

**Key Files:**
- `stores/authStore.ts` - Authentication state
- `hooks/useAuth.ts` - Auth operations
- `hooks/useSites.ts` - Site CRUD
- `hooks/useOlts.ts` - OLT CRUD
- `hooks/useUsers.ts` - User CRUD

**Dependencies:**
- Zustand
- React Query
- domain layer
- infrastructure layer

**Documentation:** [src/application/README.md](frontend/src/application/README.md)

---

#### `src/infrastructure/`
**Purpose:** External service implementations  
**Owner:** Frontend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- API repository implementations
- HTTP client configuration
- snake_case ↔ camelCase conversion

**Key Files:**
- `http/client.ts` - Axios setup with humps
- `repositories/AuthRepository.ts`
- `repositories/SiteRepository.ts`
- `repositories/UserRepository.ts`
- `repositories/OLTRepository.ts`

**Dependencies:**
- Axios
- humps (case conversion)

**Documentation:** [src/infrastructure/README.md](frontend/src/infrastructure/README.md)

---

#### `src/presentation/`
**Purpose:** UI components  
**Owner:** Frontend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- Page-level components
- Reusable UI components
- UI event handling

**Key Files:**
- `pages/` - Page components
- `components/` - Reusable components

**Dependencies:**
- Ant Design
- Ant Design Pro Components
- application layer

**Documentation:** [src/presentation/README.md](frontend/src/presentation/README.md)

---

#### `src/shared/`
**Purpose:** Shared utilities and configuration  
**Owner:** Frontend Team  
**Status:** Active  
**Last Modified:** 2026-08-15

**Responsibilities:**
- React Query client setup
- Environment variables
- Theme configuration

**Key Files:**
- `config/queryClient.ts` - React Query setup
- `config/env.ts` - Environment variables
- `theme/` - Ant Design theme customization

**Dependencies:**
- React Query
- Ant Design

**Documentation:** [src/shared/README.md](frontend/src/shared/README.md)

---

## Design Decisions Log

### DD-001: No GORM Preload Usage
**Date:** 2026-08-10  
**Status:** Active  
**Decision:** Remove all GORM relationship fields and use manual queries  
**Rationale:** GORM Preload caused cascade issues and nil pointer errors  
**Impact:** All services must manually query relationships via foreign key IDs  

### DD-002: Session-Based Authentication
**Date:** 2026-08-01  
**Status:** Active  
**Decision:** Use Redis sessions with HTTP-only cookies instead of JWT  
**Rationale:** Better security (token revocation), simpler implementation  
**Impact:** Frontend must handle cookies, no Authorization header  

### DD-003: Service Layer Owns All Business Logic
**Date:** 2026-08-01  
**Status:** Active  
**Decision:** Handlers are thin, services own all business rules  
**Rationale:** Easier testing, clear separation of concerns  
**Impact:** All validation and business logic must be in services  

### DD-004: Clean Architecture in Frontend
**Date:** 2026-08-05  
**Status:** Active  
**Decision:** Domain → Application → Infrastructure → Presentation layers  
**Rationale:** Testability, maintainability, framework independence  
**Impact:** Repository pattern required, no direct API calls from components  

### DD-005: AES-256-GCM for Credential Encryption
**Date:** 2026-08-01  
**Status:** Active  
**Decision:** Encrypt OLT credentials at rest with AES-256-GCM  
**Rationale:** Security requirement for sensitive network credentials  
**Impact:** Encryption key must be securely managed, rotation strategy needed  

---

## API Contract

See [backend/docs/API.md](backend/docs/API.md) for complete API documentation.

**Base URL:** `http://localhost:8080/api/v1`

**Authentication:** Session-based with HTTP-only cookies

**Key Endpoints:**
- `POST /auth/login` - User login
- `POST /auth/logout` - User logout
- `GET /users` - List users (Admin only)
- `GET /sites` - List sites
- `GET /olts` - List OLTs
- `POST /sites` - Create site
- `POST /olts` - Create OLT

---

## Database Schema

See [backend/docs/DATABASE.md](backend/docs/DATABASE.md) for complete schema documentation.

**Tables:**
- `users` - User accounts with roles
- `sites` - Network sites
- `olts` - OLT devices with encrypted credentials
- `audit_logs` - Audit trail

**Relationships:**
- Sites → OLTs (one-to-many via `site_id`)
- Users → Audit Logs (one-to-many via `user_id`)

---

## Security Model

1. **Authentication:** Session-based with Redis storage
2. **Authorization:** Role-based (Admin, Technician, Viewer)
3. **Encryption at Rest:** 
   - Passwords: bcrypt (cost 12)
   - OLT Credentials: AES-256-GCM
4. **Session Security:** HTTP-only cookies, secure flag in production
5. **Rate Limiting:** Applied to all API endpoints
6. **CORS:** Configured for frontend origin only

---

## Deployment Architecture

**Development:**
- Backend: localhost:8080
- Frontend: localhost:5173
- PostgreSQL: localhost:5432
- Redis: localhost:6379

**Production:**
- Docker Compose deployment
- PostgreSQL container with persistent volume
- Redis container with persistent volume
- Backend container (multi-stage build)
- Nginx reverse proxy for frontend static files

---

## Testing Strategy

**Backend:**
- Unit tests for services
- Integration tests with in-memory SQLite
- Race detector enabled in CI
- Coverage target: 80%+

**Frontend:**
- Unit tests with Vitest
- Component tests with Testing Library
- Mock API calls in tests
- Coverage target: 70%+

---

## CI/CD Pipeline

See [CLAUDE.md](CLAUDE.md) for CI/CD details.

**Workflows:**
1. Backend CI - test, lint, build
2. Frontend CI - test, lint, format, build
3. Docker Build - build and push images
4. Code Quality - security scanning
5. Deploy - automated deployment

---

## Maintenance Notes

**Regular Tasks:**
- Update dependencies monthly
- Review audit logs weekly
- Rotate encryption keys quarterly
- Database backup daily (production)
- Monitor session store size

**Known Limitations:**
- Single Redis instance (no HA)
- No multi-tenancy support yet
- Manual OLT credential rotation
- No real-time monitoring dashboard

---

## Change Log

| Date | Module | Change | Author |
|------|--------|--------|--------|
| 2026-08-15 | SOT | Initial SOT creation | System |
| 2026-08-10 | internal/models | Removed GORM relationships | Backend |
| 2026-08-10 | internal/services | Updated to manual queries | Backend |

---

## Future Roadmap

**Q3 2026:**
- Real-time OLT monitoring dashboard
- Bulk OLT provisioning
- Report generation module

**Q4 2026:**
- Multi-tenancy support
- Advanced RBAC with custom permissions
- Integration with external SNMP tools

**2027:**
- Mobile app (React Native)
- High availability Redis cluster
- Automated credential rotation

