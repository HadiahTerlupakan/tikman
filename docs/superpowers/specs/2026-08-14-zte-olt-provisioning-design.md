# ZTE OLT Provisioning Application - Design Specification

**Date**: 2026-08-14  
**Project**: TikMan - ZTE OLT Provisioning System  
**Version**: 1.0

## 1. Overview

Aplikasi web untuk provisioning dan monitoring ZTE OLT (Optical Line Terminal) dengan kemampuan multi-site management, role-based access control, dan real-time monitoring. Aplikasi menggunakan SSH/Telnet untuk command execution dan SNMP v2c untuk monitoring.

### Key Features
- Multi-site OLT management
- Auto-discovery unconfirmed ONTs
- ONT provisioning dengan service/line profile management
- Real-time monitoring dengan WebSocket
- Role-based access control (Admin, Technician, Viewer)
- Comprehensive audit logging
- Dockerized deployment dengan PostgreSQL dan Redis

## 2. System Architecture

### Architecture Pattern
Microservices-Lite dengan separation between API layer dan background worker untuk handling long-running OLT operations.

### Components

**API Service (Go)**
- REST API untuk CRUD operations
- Session-based authentication menggunakan Redis
- WebSocket server untuk real-time updates
- Role-based authorization middleware
- Port: 8080 (internal)

**Worker Service (Go)**
- Background job processor
- SSH/Telnet/SNMP operations ke OLT
- Periodic polling untuk monitoring
- Auto-discovery scheduler
- Concurrent job processing dengan rate limiting

**Frontend (React + TypeScript)**
- Single Page Application
- Real-time dashboard
- Ant Design component library
- WebSocket client untuk live updates
- Port: 3000 (served by Nginx)

**PostgreSQL**
- Primary database
- Stores: users, sites, OLTs, ONTs, profiles, audit logs
- Port: 5432 (internal only)

**Redis**
- Job queue untuk worker tasks
- Session storage
- Caching layer
- Port: 6379 (internal only)

**Nginx**
- Reverse proxy
- Static file serving
- Rate limiting
- Port: 80 (exposed)

### Communication Flow
```
User Browser (React)
    ↓ HTTPS/WebSocket
Nginx :80
    ↓ HTTP
API Service :8080
    ↓ Redis Queue
Worker Service
    ↓ SSH/Telnet/SNMP
ZTE OLT Devices
```

## 3. Database Schema

### users
```sql
id              uuid PRIMARY KEY
username        varchar UNIQUE NOT NULL
email           varchar UNIQUE NOT NULL
password_hash   varchar NOT NULL
role            enum('admin', 'technician', 'viewer') NOT NULL
created_at      timestamp DEFAULT now()
updated_at      timestamp DEFAULT now()
```

### sites
```sql
id              uuid PRIMARY KEY
name            varchar NOT NULL
location        text
description     text
created_at      timestamp DEFAULT now()
updated_at      timestamp DEFAULT now()
```

### olts
```sql
id                  uuid PRIMARY KEY
site_id             uuid REFERENCES sites(id) ON DELETE CASCADE
name                varchar NOT NULL
ip_address          varchar NOT NULL
ssh_port            int DEFAULT 22
telnet_port         int DEFAULT 23
snmp_port           int DEFAULT 161
snmp_community      varchar DEFAULT 'public'
preferred_protocol  enum('ssh', 'telnet') DEFAULT 'ssh'
username            varchar NOT NULL
password            varchar NOT NULL  -- encrypted with AES-256
status              enum('online', 'offline', 'error') DEFAULT 'offline'
last_seen           timestamp
created_at          timestamp DEFAULT now()
updated_at          timestamp DEFAULT now()
```

### service_profiles
```sql
id              uuid PRIMARY KEY
olt_id          uuid REFERENCES olts(id) ON DELETE CASCADE
profile_name    varchar NOT NULL
profile_id      int NOT NULL  -- ID di OLT device
description     text
created_at      timestamp DEFAULT now()
updated_at      timestamp DEFAULT now()
UNIQUE(olt_id, profile_id)
```

### line_profiles
```sql
id              uuid PRIMARY KEY
olt_id          uuid REFERENCES olts(id) ON DELETE CASCADE
profile_name    varchar NOT NULL
profile_id      int NOT NULL  -- ID di OLT device
bandwidth_down  int  -- Mbps
bandwidth_up    int  -- Mbps
vlan_id         int
description     text
created_at      timestamp DEFAULT now()
updated_at      timestamp DEFAULT now()
UNIQUE(olt_id, profile_id)
```

### onts
```sql
id                      uuid PRIMARY KEY
olt_id                  uuid REFERENCES olts(id) ON DELETE CASCADE
serial_number           varchar NOT NULL
pon_port                varchar NOT NULL  -- format: "0/1/2" (frame/slot/port)
ont_id                  int NOT NULL
service_profile_id      uuid REFERENCES service_profiles(id)
line_profile_id         uuid REFERENCES line_profiles(id)
customer_name           varchar
description             text
status                  enum('online', 'offline', 'los', 'dying_gasp', 'unconfirmed', 'pending') DEFAULT 'pending'
signal_rx               float  -- dBm
signal_tx               float  -- dBm
distance                int  -- meter
last_online             timestamp
created_at              timestamp DEFAULT now()
updated_at              timestamp DEFAULT now()
INDEX(serial_number)
INDEX(olt_id, status)
UNIQUE(olt_id, pon_port, ont_id)
```

### audit_logs
```sql
id              uuid PRIMARY KEY
user_id         uuid REFERENCES users(id) ON DELETE SET NULL
action          varchar NOT NULL  -- 'create_ont', 'update_ont', 'delete_ont', etc
resource_type   varchar NOT NULL  -- 'ont', 'olt', 'profile', 'user'
resource_id     uuid
old_value       jsonb  -- before state
new_value       jsonb  -- after state
ip_address      varchar
user_agent      text
created_at      timestamp DEFAULT now()
INDEX(user_id, created_at)
INDEX(resource_type, resource_id)
```

## 4. API Endpoints

### Authentication
```
POST   /api/v1/auth/login       - Login user, create session
POST   /api/v1/auth/logout      - Logout, destroy session
GET    /api/v1/auth/me          - Get current user info
```

### Users Management (Admin only)
```
GET    /api/v1/users            - List all users
POST   /api/v1/users            - Create new user
PUT    /api/v1/users/:id        - Update user
DELETE /api/v1/users/:id        - Delete user
```

### Sites
```
GET    /api/v1/sites            - List all sites
POST   /api/v1/sites            - Create site (Admin/Technician)
PUT    /api/v1/sites/:id        - Update site (Admin/Technician)
DELETE /api/v1/sites/:id        - Delete site (Admin only)
```

### OLTs
```
GET    /api/v1/olts             - List all OLTs with status
GET    /api/v1/olts/:id         - Get OLT detail
POST   /api/v1/olts             - Add new OLT (Admin/Technician)
PUT    /api/v1/olts/:id         - Update OLT config (Admin/Technician)
DELETE /api/v1/olts/:id         - Delete OLT (Admin only)
POST   /api/v1/olts/:id/test    - Test OLT connectivity
```

### Service & Line Profiles
```
GET    /api/v1/olts/:oltId/service-profiles      - List service profiles
POST   /api/v1/olts/:oltId/service-profiles/sync - Sync from OLT
GET    /api/v1/olts/:oltId/line-profiles         - List line profiles
POST   /api/v1/olts/:oltId/line-profiles/sync    - Sync from OLT
```

### ONTs
```
GET    /api/v1/onts             - List all ONTs (filters: olt_id, status, search)
GET    /api/v1/onts/:id         - Get ONT detail
POST   /api/v1/onts/discover    - Trigger auto-discovery on OLT
POST   /api/v1/onts             - Provision new ONT (Admin/Technician)
PUT    /api/v1/onts/:id         - Update ONT config (Admin/Technician)
DELETE /api/v1/onts/:id         - Deactivate/delete ONT (Admin/Technician)
POST   /api/v1/onts/:id/refresh - Force refresh ONT status
```

### Dashboard & Monitoring
```
GET    /api/v1/dashboard/overview        - Overview stats
GET    /api/v1/dashboard/recent-activity - Recent provisioning activity
WS     /api/v1/ws                        - WebSocket for real-time updates
```

### Audit Logs
```
GET    /api/v1/audit-logs       - List audit logs (with filters)
```

### Health Check
```
GET    /health                  - Service health status
```

## 5. Worker Service & Job Types

### Job Queue Architecture
Worker consumes jobs dari Redis queue dengan concurrent processing dan rate limiting.

### Job Types

**ont_discovery**
- Description: Auto-discover unconfirmed ONTs di OLT
- Payload: `{olt_id: uuid, protocol: "ssh"|"telnet"}`
- Schedule: Periodic (every 5 minutes per OLT) atau on-demand
- Priority: Low

**ont_provision**
- Description: Provision ONT baru
- Payload: `{olt_id, serial_number, pon_port, ont_id, service_profile_id, line_profile_id, customer_name, description}`
- Trigger: User action via API
- Priority: High

**ont_update**
- Description: Update ONT configuration
- Payload: `{ont_id, updates: {...}}`
- Trigger: User action
- Priority: High

**ont_delete**
- Description: Deactivate/remove ONT
- Payload: `{ont_id}`
- Trigger: User action
- Priority: High

**ont_status_poll**
- Description: Poll ONT status (signal, distance, uptime) via SNMP
- Payload: `{olt_id, ont_ids: [uuid]}`
- Schedule: Periodic (every 30 seconds per batch)
- Priority: Medium

**olt_health_check**
- Description: Check OLT connectivity (SSH/Telnet/SNMP)
- Payload: `{olt_id}`
- Schedule: Periodic (every 1 minute)
- Priority: High

**profile_sync**
- Description: Sync service/line profiles from OLT to database
- Payload: `{olt_id, profile_type: "service"|"line"}`
- Trigger: On-demand via API
- Priority: Medium

### Worker Configuration
- Concurrent workers: 5-10 goroutines per worker instance
- Rate limiting: Max 2 concurrent connections per OLT
- Retry policy: 3 attempts dengan exponential backoff (1s, 2s, 4s)
- Timeout: 30 seconds per SSH/Telnet command
- Dead letter queue: Failed jobs after max retries
- Scalability: Worker dapat di-scale horizontal via docker-compose

### WebSocket Broadcast
Setelah job completion, worker broadcast update ke connected clients via API service WebSocket.

## 6. Frontend Structure

### Tech Stack
- React 18 + TypeScript
- React Router - routing
- TanStack Query (React Query) - server state management
- Zustand - client state management
- Axios - HTTP client
- Native WebSocket API - real-time updates
- Ant Design - UI component library
- Recharts - dashboard charting
- Tailwind CSS - utility styling
- Vite - build tool

### Pages & Routes

**Public Routes**
- `/login` - Login page

**Protected Routes**
- `/` - Dashboard (overview, stats, recent activity)
- `/sites` - Sites management
- `/olts` - OLTs management
- `/onts` - ONTs listing dengan filters
- `/onts/discover` - Auto-discovery page
- `/profiles` - Service & Line profiles management
- `/audit-logs` - Audit logs viewer
- `/users` - Users management (Admin only)

### Dashboard Components

**Overview Cards**
- Total OLTs (online/offline)
- Total ONTs (online/offline/los/dying_gasp)
- Per-site statistics

**Site Summary Table**
- Site name, location
- OLT count, ONT count
- Online/offline status
- Drill-down ke detail

**Recent Activity Timeline**
- Last 20 provisioning actions
- User, action, timestamp
- Link ke resource

**Real-time Indicators**
- Green/Red status badges
- Signal quality color-coded (> -25 dBm green, -25 to -28 yellow, < -28 red)
- WebSocket connection status

### ONTs Management Features
- Table dengan filters: site, OLT, status, search (SN/customer name)
- Status badges: online (green), offline (red), los (orange), dying_gasp (red blink)
- Signal RX/TX display dengan color indicators
- Quick provision button → modal form
- Bulk actions: refresh selected ONTs, export to CSV
- Pagination: 50 items per page

### ONT Discovery Workflow
1. Select OLT dari dropdown
2. Click "Start Discovery" → trigger API job
3. Loading state dengan progress
4. Display unconfirmed ONTs table dengan serial numbers
5. Quick authorize button per ONT → modal form prefilled dengan SN
6. Batch authorization: select multiple → assign same profile

### Real-time Updates
- WebSocket connection on app mount
- Auto-reconnect on disconnect
- Message handlers untuk different event types
- Toast notifications untuk job completion
- Live status updates di tables tanpa full refresh

## 7. Technology Stack

### Backend (Go)
- **Framework**: Gin - HTTP web framework
- **ORM**: GORM - PostgreSQL ORM
- **SSH**: `golang.org/x/crypto/ssh` - SSH client
- **Telnet**: `ziutek/telnet` - Telnet client
- **SNMP**: `gosnmp/gosnmp` - SNMP v2c library
- **Redis**: `go-redis/redis` - Redis client
- **WebSocket**: `gorilla/websocket` - WebSocket implementation
- **Auth**: Session-based dengan Redis storage
- **Config**: Viper - configuration management
- **Logging**: Zap - structured, leveled logging
- **Migration**: golang-migrate - database migrations
- **Testing**: testify - assertion library

### Frontend (React)
- **UI Library**: Ant Design - comprehensive component library
- **State Management**: TanStack Query (server), Zustand (client)
- **HTTP Client**: Axios dengan interceptors
- **Routing**: React Router v6
- **Charting**: Recharts
- **Styling**: Tailwind CSS
- **Build**: Vite
- **Testing**: Vitest + React Testing Library

### Infrastructure
- **Database**: PostgreSQL 15
- **Cache/Queue**: Redis 7
- **Reverse Proxy**: Nginx
- **Container**: Docker + Docker Compose
- **CI/CD**: GitHub Actions

### Development Tools
- **Hot Reload (Go)**: Air
- **Code Format (Go)**: gofmt, goimports
- **Linting (Go)**: golangci-lint
- **Linting (TS)**: ESLint + Prettier

## 8. Security

### Authentication & Authorization

**Session-Based Authentication**
- Random secure session token (UUID v4)
- Session data stored di Redis dengan TTL 24 jam
- Sliding window: TTL refresh pada setiap request
- Token delivered via secure HTTP-only cookie
- Logout = instant session invalidation dari Redis

**Session Storage (Redis)**
- Key: `session:{token}`
- Value: `{user_id, role, created_at, last_activity}`
- TTL: 24 hours dengan auto-refresh

**Cookie Configuration**
- HttpOnly: true (prevent XSS)
- Secure: true (HTTPS only)
- SameSite: Strict (CSRF protection)
- Path: /api

**Role-Based Access Control (RBAC)**
- Admin: full access
- Technician: read/write OLTs, ONTs, profiles; read sites, users
- Viewer: read-only access

### Credential Storage
- OLT passwords encrypted dengan AES-256-GCM
- Encryption key dari environment variable
- Key rotation support (nanti phase 2)
- User passwords hashed dengan bcrypt (cost 12)

### API Security
- Rate limiting: 100 requests/minute per IP
- Input validation untuk semua endpoints (binding + custom validators)
- SQL injection prevention via GORM parameterized queries
- XSS prevention: React auto-escaping + CSP headers
- CORS configuration dengan whitelist origins

### Network Security
- Docker internal network untuk inter-service communication
- Only Nginx port 80 exposed ke host
- PostgreSQL dan Redis tidak accessible dari luar container network
- Optional: SSH key-based authentication untuk OLT (phase 2)

### Logging & Audit
- Audit log untuk semua mutating operations
- Log user_id, IP address, user agent, timestamp
- Before/after state untuk traceability
- Structured logging dengan correlation IDs

## 9. Error Handling

### Backend Error Responses
**Standard Error Format**
```json
{
  "error": "Human-readable error message",
  "code": "ERROR_CODE",
  "details": {}  // optional additional context
}
```

**HTTP Status Codes**
- 400: Bad Request (validation errors)
- 401: Unauthorized (not authenticated)
- 403: Forbidden (not authorized)
- 404: Not Found
- 409: Conflict (duplicate resource)
- 429: Too Many Requests (rate limit)
- 500: Internal Server Error
- 503: Service Unavailable (dependency down)

**Error Categories**
- User errors (4xx): return actionable message
- System errors (5xx): log full stack, return generic message

### Worker Error Handling
- Job retry mechanism: 3 attempts dengan exponential backoff
- Dead letter queue untuk persistent failures
- Alert logging untuk failed jobs setelah max retries
- Timeout handling: 30s per operation, graceful connection close
- Partial failure handling: 1 ONT gagal tidak affect batch lainnya

### Frontend Error Handling
- Error boundaries untuk prevent app crash
- User-friendly error messages (no raw stack traces)
- Toast notifications untuk operation results
- Loading states untuk async operations
- Retry buttons untuk failed operations
- WebSocket reconnection dengan backoff

### Graceful Degradation
- Jika 1 OLT offline, yang lain tetap accessible
- Jika worker down, API tetap serve read operations
- Jika Redis down, API fallback ke database session (optional)
- Display stale data dengan warning jika real-time update failed

## 10. Data Flow Examples

### ONT Provisioning Flow
1. User submit form di UI (SN, PON port, profiles, customer info)
2. Frontend → `POST /api/v1/onts`
3. API validates input, create ONT record dengan status "pending"
4. API write audit log
5. API enqueue job ke Redis: `ont_provision` dengan payload
6. API return response: `{ont_id, status: "pending"}`
7. Worker consume job dari queue
8. Worker establish SSH/Telnet connection ke OLT
9. Worker execute provisioning commands
10. Worker update ONT status: "online" atau "error"
11. Worker write audit log (success/failure)
12. Worker broadcast via WebSocket: `{type: "ont_updated", ont_id, status, ...}`
13. Frontend receive WebSocket message → update UI

### Auto-Discovery Flow
1. Worker schedule periodic job (every 5 min per OLT)
2. Worker connect SSH/Telnet ke OLT
3. Worker execute: `show gpon onu uncfg`
4. Parse output: extract serial numbers
5. Query database: filter yang belum exist
6. Bulk insert ONT records dengan status "unconfirmed"
7. Broadcast via WebSocket: `{type: "unconfirmed_onts_found", olt_id, count}`
8. Frontend show toast notification
9. User navigate ke Discovery page → see list
10. User select ONT → click authorize → provision flow

### Monitoring/Polling Flow
1. Worker schedule periodic job (every 30 sec)
2. Worker query batch ONTs via SNMP (faster than SSH for bulk reads)
3. Parse SNMP OIDs: signal RX/TX, status, distance
4. Bulk update database
5. Broadcast via WebSocket: `{type: "onts_batch_update", data: [{ont_id, status, signal_rx, signal_tx, distance}]}`
6. Frontend merge updates ke current table state
7. UI update status indicators dan signal values

### WebSocket Message Types
```typescript
type WSMessage = 
  | { type: "ont_created", data: ONT }
  | { type: "ont_updated", data: Partial<ONT> & { id: string } }
  | { type: "ont_deleted", data: { id: string } }
  | { type: "olt_status_changed", data: { id: string, status: OLTStatus } }
  | { type: "onts_batch_update", data: Array<Partial<ONT> & { id: string }> }
  | { type: "job_completed", data: { job_id: string, status: "success"|"failed", message: string } }
  | { type: "unconfirmed_onts_found", data: { olt_id: string, count: number } }
```

## 11. Deployment

### Docker Compose Structure
```yaml
version: '3.9'

services:
  postgres:
    image: postgres:15-alpine
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: tikman
      POSTGRES_USER: tikman
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U tikman"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - tikman-network
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - ./data/redis:/data
    healthcheck:
      test: ["CMD", "redis-cli", "--raw", "incr", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - tikman-network
    restart: unless-stopped

  api:
    build:
      context: ./backend
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=tikman
      - DB_PASSWORD=${POSTGRES_PASSWORD}
      - DB_NAME=tikman
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - SESSION_SECRET=${SESSION_SECRET}
      - LOG_LEVEL=info
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
    networks:
      - tikman-network
    restart: unless-stopped

  worker:
    build:
      context: ./backend
      dockerfile: Dockerfile.worker
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=tikman
      - DB_PASSWORD=${POSTGRES_PASSWORD}
      - DB_NAME=tikman
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - LOG_LEVEL=info
      - POLLING_INTERVAL_ONT_STATUS=30
      - POLLING_INTERVAL_OLT_HEALTH=60
      - POLLING_INTERVAL_DISCOVERY=300
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - tikman-network
    restart: unless-stopped
    deploy:
      replicas: 2  # scalable

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    networks:
      - tikman-network

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - api
      - frontend
    networks:
      - tikman-network
    restart: unless-stopped

networks:
  tikman-network:
    driver: bridge

volumes:
  postgres_data:
  redis_data:
```

### Environment Variables (.env)
```bash
# Database
POSTGRES_PASSWORD=<strong-password>

# Redis
REDIS_PASSWORD=<strong-password>

# Security
ENCRYPTION_KEY=<32-byte-random-key>  # for AES-256
SESSION_SECRET=<random-secret>

# Optional: Custom polling intervals (seconds)
POLLING_INTERVAL_ONT_STATUS=30
POLLING_INTERVAL_OLT_HEALTH=60
POLLING_INTERVAL_DISCOVERY=300
```

### Initial Setup
1. Clone repository
2. Copy `.env.example` to `.env`, fill values
3. Run: `docker-compose up -d`
4. Database migration runs automatically on API startup
5. Default admin user seeded: `admin / changeme123`
6. Access: `http://localhost`

### GitHub Actions CI/CD
**Workflow**: `.github/workflows/ci.yml`
- Trigger: push to main, pull requests
- Jobs:
  - **Lint**: golangci-lint (backend), ESLint (frontend)
  - **Test**: Go unit tests, React component tests
  - **Build**: docker build untuk semua services
  - **Deploy** (optional): push images ke registry, deploy ke server

## 12. Testing Strategy

### Backend Testing

**Unit Tests**
- Service layer: provisioning logic, validation
- Utilities: encryption, OLT command parsers
- Middleware: auth, RBAC, rate limiting
- Target coverage: 70%+ untuk business logic
- Run: `go test ./...`

**Integration Tests**
- API endpoints dengan testcontainers (PostgreSQL, Redis)
- Worker job processing dengan mock SSH/Telnet
- Session management flow
- Run: `go test -tags=integration ./...`

### Frontend Testing
- Component tests: forms, tables, filters
- Critical flows: login, provision ONT, discovery
- Tool: Vitest + React Testing Library
- Run: `npm test`

### Manual Testing Priority
- Real-time WebSocket updates dengan multiple clients
- Multi-user concurrent operations
- OLT connection handling (SSH fallback ke Telnet)
- Signal quality monitoring accuracy

### Development Workflow
1. **Local Development**
   - `docker-compose up postgres redis` (infra only)
   - Run API: `cd backend && air` (hot reload)
   - Run worker: `cd backend && go run cmd/worker/main.go`
   - Run frontend: `cd frontend && npm run dev`
   - Mock OLT responses untuk development tanpa hardware

2. **Testing**
   - Unit tests run di pre-commit hook
   - Integration tests run di GitHub Actions
   - E2E tests (optional, phase 2)

3. **Production**
   - Full docker-compose stack
   - Health checks dan auto-restart
   - Logs ke stdout (integrasi dengan external logging optional)

## 13. Future Enhancements (Out of Scope for MVP)

### Phase 2 Features
- Bulk ONT provisioning via CSV import
- OLT firmware upgrade management
- Advanced analytics dashboard dengan historical trends
- Email/Slack notifications untuk alerts
- Multi-tenancy support
- API rate limiting per user
- GraphQL API option
- Mobile app (React Native)

### Scalability Improvements
- Horizontal scaling dengan load balancer
- Read replicas untuk PostgreSQL
- Redis Cluster untuk high availability
- Prometheus metrics + Grafana dashboards
- Distributed tracing (OpenTelemetry)

### Security Enhancements
- 2FA authentication
- API key management untuk external integrations
- Encryption key rotation
- SSH key-based OLT authentication
- Penetration testing dan security audit

## 14. Success Criteria

### MVP Acceptance
- [x] User dapat login dengan role-based access
- [x] User dapat add/edit/delete OLT dengan test connection
- [x] User dapat auto-discover unconfirmed ONTs
- [x] User dapat provision ONT baru dengan service/line profile
- [x] Dashboard menampilkan real-time status ONT (online/offline)
- [x] Signal quality monitoring via SNMP
- [x] Audit log mencatat semua provisioning actions
- [x] Multi-site management
- [x] Dockerized deployment yang reproducible

### Performance Targets
- API response time: < 200ms untuk read operations
- ONT provisioning time: < 5 seconds per ONT
- WebSocket latency: < 1 second untuk status updates
- Support minimal 50 OLTs dan 5000 ONTs concurrent
- Worker dapat handle 10 concurrent jobs tanpa degradasi

### Reliability Targets
- API uptime: 99.5%
- Worker job success rate: > 95%
- Automatic recovery dari connection failures
- Zero data loss untuk provisioning operations

## 15. Project Structure

```
tikman/
├── backend/
│   ├── cmd/
│   │   ├── api/main.go
│   │   └── worker/main.go
│   ├── internal/
│   │   ├── api/          # HTTP handlers
│   │   ├── auth/         # Session management
│   │   ├── config/       # Configuration
│   │   ├── database/     # DB connection, migrations
│   │   ├── middleware/   # Auth, RBAC, rate limiting
│   │   ├── models/       # GORM models
│   │   ├── olt/          # SSH/Telnet/SNMP clients
│   │   ├── queue/        # Redis job queue
│   │   ├── services/     # Business logic
│   │   ├── utils/        # Helpers, encryption
│   │   └── websocket/    # WebSocket hub
│   ├── migrations/       # SQL migrations
│   ├── tests/
│   ├── Dockerfile
│   ├── Dockerfile.worker
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/   # Reusable components
│   │   ├── pages/        # Route pages
│   │   ├── services/     # API client, WebSocket
│   │   ├── hooks/        # Custom React hooks
│   │   ├── store/        # Zustand stores
│   │   ├── types/        # TypeScript types
│   │   ├── utils/        # Helpers
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── public/
│   ├── Dockerfile
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── nginx/
│   └── nginx.conf
├── docs/
│   └── superpowers/
│       └── specs/
│           └── 2026-08-14-zte-olt-provisioning-design.md
├── .github/
│   └── workflows/
│       └── ci.yml
├── docker-compose.yml
├── .env.example
├── .gitignore
└── README.md
```

## 16. Glossary

- **OLT**: Optical Line Terminal - perangkat di sisi operator untuk terminate fiber optic
- **ONT/ONU**: Optical Network Terminal/Unit - perangkat di sisi pelanggan
- **PON**: Passive Optical Network - port di OLT untuk connect multiple ONTs
- **Service Profile**: Template konfigurasi service (internet, voice, IPTV)
- **Line Profile**: Template konfigurasi bandwidth dan VLAN
- **LOS**: Loss of Signal - ONT kehilangan signal dari OLT
- **Dying Gasp**: Alarm ketika ONT kehilangan power
- **Unconfirmed ONT**: ONT yang terdeteksi tapi belum di-authorize
- **Signal RX**: Received signal strength di ONT (from OLT)
- **Signal TX**: Transmitted signal strength dari ONT (to OLT)

---

**End of Design Document**
