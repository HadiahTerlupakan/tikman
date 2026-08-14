# TikMan - ZTE OLT Provisioning System

Web-based application for provisioning and monitoring ZTE OLT (Optical Line Terminal) devices.

## Features

- Multi-site OLT management
- ONT provisioning with SSH/Telnet
- Real-time monitoring via SNMP
- Role-based access control (Admin, Technician, Viewer)
- Comprehensive audit logging

## Tech Stack

**Backend:**
- Go 1.23+ with Gin framework
- PostgreSQL 15
- Redis 7
- GORM ORM

**Frontend:** (Coming in Phase 2)
- React 18 + TypeScript
- Ant Design
- WebSocket for real-time updates

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Git

### Development Setup

1. **Clone repository**

```bash
git clone <repository-url>
cd tikman
```

2. **Copy environment file**

```bash
cp .env.example .env
```

3. **Edit .env file** - Update passwords and keys

4. **Start infrastructure** (PostgreSQL + Redis)

```bash
docker-compose -f docker-compose.dev.yml up -d
```

5. **Run API locally** (with hot reload)

```bash
cd backend
go mod download
go install github.com/cosmtrek/air@latest
air
```

API will be available at `http://localhost:8080`

### Production Deployment

1. **Set environment variables**

```bash
cp .env.example .env
# Edit .env with production values
```

2. **Start all services**

```bash
docker-compose up -d
```

3. **Check health**

```bash
curl http://localhost:8080/health
```

### Default Credentials

- **Username:** `admin`
- **Password:** `changeme123`

**⚠️ Change the default password immediately after first login!**

## API Endpoints

### Authentication
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/logout` - Logout
- `GET /api/v1/auth/me` - Get current user

### Users (Admin only)
- `GET /api/v1/users` - List users
- `POST /api/v1/users` - Create user
- `PUT /api/v1/users/:id` - Update user
- `DELETE /api/v1/users/:id` - Delete user

### Sites
- `GET /api/v1/sites` - List sites
- `POST /api/v1/sites` - Create site (Admin/Technician)
- `GET /api/v1/sites/:id` - Get site
- `PUT /api/v1/sites/:id` - Update site (Admin/Technician)
- `DELETE /api/v1/sites/:id` - Delete site (Admin)

### OLTs
- `GET /api/v1/olts` - List OLTs
- `POST /api/v1/olts` - Create OLT (Admin/Technician)
- `GET /api/v1/olts/:id` - Get OLT
- `PUT /api/v1/olts/:id` - Update OLT (Admin/Technician)
- `DELETE /api/v1/olts/:id` - Delete OLT (Admin)

## Testing

Run unit tests:

```bash
cd backend
go test ./... -v
```

Run with coverage:

```bash
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Project Structure

```
tikman/
├── backend/
│   ├── cmd/
│   │   ├── api/          # API server entry point
│   │   └── worker/       # Worker service (Phase 2)
│   ├── internal/
│   │   ├── api/          # HTTP handlers & DTOs
│   │   ├── auth/         # Session management
│   │   ├── config/       # Configuration
│   │   ├── database/     # DB connection
│   │   ├── logger/       # Structured logging
│   │   ├── middleware/   # Auth, RBAC, rate limiting
│   │   ├── models/       # GORM models
│   │   ├── services/     # Business logic
│   │   └── utils/        # Helpers, encryption
│   ├── Dockerfile
│   └── go.mod
├── docker-compose.yml
├── docker-compose.dev.yml
├── .env.example
└── README.md
```

## Development

### Running Tests

```bash
cd backend
go test ./internal/... -v
```

### Code Formatting

```bash
go fmt ./...
goimports -w .
```

### Linting

```bash
golangci-lint run
```

## Security

- Passwords hashed with bcrypt (cost 12)
- OLT credentials encrypted with AES-256-GCM
- Session-based auth with Redis
- HTTP-only cookies
- RBAC on all endpoints

## License

Proprietary - All rights reserved

## Support

For issues and questions, contact the development team.
