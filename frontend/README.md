# TikMan Frontend

React + TypeScript frontend for ZTE OLT Provisioning System.

## Tech Stack

- React 18 + TypeScript 5
- Vite 5 (build tool)
- Ant Design 5 (UI library)
- React Router v7 (routing)
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

# Run tests (bare `npm test` is watch mode)
npm test -- --run

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

Push notifications need the Firebase web config. Vite inlines these at build
time, so `docker-compose.yml` passes them as build args rather than runtime
environment — leaving them unset builds an image with push permanently inert:

- `VITE_FIREBASE_API_KEY`
- `VITE_FIREBASE_AUTH_DOMAIN`
- `VITE_FIREBASE_PROJECT_ID`
- `VITE_FIREBASE_STORAGE_BUCKET`
- `VITE_FIREBASE_MESSAGING_SENDER_ID`
- `VITE_FIREBASE_APP_ID`
- `VITE_FIREBASE_VAPID_KEY`

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
