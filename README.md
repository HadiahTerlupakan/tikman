# TikMan

Network device management and monitoring system with web-based interface.

## Features

- Multi-site device management
- Real-time monitoring via SNMP
- Role-based access control
- Comprehensive audit logging
- RESTful API

## Tech Stack

**Backend:**
- Go 1.25+ with Gin framework
- PostgreSQL 15
- Redis 7

**Frontend:**
- React 18 + TypeScript
- Ant Design
- React Query

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Git

### Development

```bash
# Clone repository
git clone <repository-url>
cd tikman

# Setup environment
cp .env.example .env

# Start services
docker-compose up -d

# API available at http://localhost:8080
# Frontend at http://localhost:3000
```

### Default Credentials

- Username: `admin`
- Password: `admin123`

**⚠️ Change default password after first login**

## Project Structure

```
tikman/
├── backend/          # Go API & Worker
├── frontend/         # React TypeScript UI
├── docs/            # Documentation
├── scripts/         # Maintenance scripts
└── data/            # Data storage
```

## Development

### Backend

```bash
cd backend
go mod download
go test ./...
air  # hot reload
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

## Testing

```bash
# Backend tests
cd backend && go test ./... -v

# Frontend tests (bare `npm test` is watch mode)
cd frontend && npm test -- --run
```

## Documentation

- [Security Guidelines](docs/SECURITY.md)
- [Scripts Guide](scripts/README.md)
- [Operator Guide](docs/operator_guide.md)
- [API Reference](docs/api_reference.md)
- [Architecture Notes](CLAUDE.md)

## License

Proprietary - All rights reserved
