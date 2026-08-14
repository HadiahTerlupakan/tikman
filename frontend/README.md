# TikMan Frontend

React + TypeScript frontend for ZTE OLT Provisioning System.

## Tech Stack

- React 18 + TypeScript 5
- Vite 5 (build tool)
- Ant Design 5 (UI library)
- React Router v6 (routing)
- React Query (server state)
- Zustand (client state)
- Axios (HTTP client)

## Architecture

Clean Architecture with 4 layers:
- **Domain**: Pure TypeScript entities and interfaces
- **Infrastructure**: API client and repository implementations
- **Application**: Hooks and state management
- **Presentation**: React components and pages

## Development

```bash
# Install dependencies
npm install

# Run dev server
npm run dev

# Run tests
npm test

# Build for production
npm run build

# Preview production build
npm run preview
```

## Docker

```bash
# Build image
docker build -t tikman-frontend .

# Run container
docker run -p 3000:80 tikman-frontend
```

## Environment Variables

- `VITE_API_URL`: Backend API URL (default: http://localhost:8080)
- `VITE_APP_NAME`: Application name

## Project Structure

```
src/
├── domain/           # Entities and repository interfaces
├── infrastructure/   # API client and implementations
├── application/      # Hooks and state management
├── presentation/     # React components and pages
├── shared/          # Shared utilities and config
└── test/            # Test setup and utilities
```
