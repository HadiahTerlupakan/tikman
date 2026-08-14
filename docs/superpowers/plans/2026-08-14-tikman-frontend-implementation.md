# TikMan Frontend - React + TypeScript Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build production-ready React frontend dengan Clean Architecture untuk ZTE OLT Provisioning System

**Architecture:** Clean Architecture dengan 4 layers - Domain (pure TS entities), Infrastructure (API client), Application (hooks/contexts), Presentation (React components). Session-based auth via cookies, React Query untuk server state, Zustand untuk client state.

**Tech Stack:** React 18, TypeScript 5, Vite 5, Ant Design 5, React Router v6, React Query, Zustand, Axios, Vitest

## Global Constraints

- React: 18.3+
- TypeScript: 5.3+ (strict mode)
- Node: 20+
- Vite: 5.1+
- Ant Design: 5.15+
- API Backend: http://localhost:8080
- Session auth via HttpOnly cookies
- Clean Architecture: Domain → Infrastructure → Application → Presentation
- File naming: PascalCase for components, camelCase for utils, useCamelCase for hooks
- All imports use path aliases (@/domain, @/infrastructure, @/application, @/presentation, @/shared)
- TDD workflow: test first, verify fail, implement, verify pass, commit
- Error format from API: `{"error": string, "code": string, "details": object}`
- All API calls withCredentials: true (for cookies)
- Bundle target: Initial < 200KB gzipped

---

### Task 1: Project Scaffolding & Configuration

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/tsconfig.json`
- Create: `frontend/tsconfig.node.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/.env.example`
- Create: `frontend/.env.development`
- Create: `frontend/.gitignore`
- Create: `frontend/index.html`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/vite-env.d.ts`

**Interfaces:**
- Consumes: None (first task)
- Produces: 
  - Vite dev server running on port 3000
  - TypeScript configured with strict mode + path aliases
  - Basic React app renders "Hello World"

- [ ] **Step 1: Initialize Node project**

```bash
mkdir -p frontend
cd frontend
npm init -y
```

- [ ] **Step 2: Install core dependencies**

```bash
npm install react@18.3.1 react-dom@18.3.1
npm install react-router-dom@6.22.3
npm install antd@5.15.4 @ant-design/icons@5.3.6
npm install @tanstack/react-query@5.24.8
npm install zustand@4.5.2
npm install axios@1.6.8
```

- [ ] **Step 3: Install dev dependencies**

```bash
npm install -D typescript@5.3.3
npm install -D @types/react@18.2.66 @types/react-dom@18.2.22
npm install -D @types/node@20.11.30
npm install -D vite@5.1.6 @vitejs/plugin-react-swc@3.6.0
npm install -D vitest@1.3.1 @testing-library/react@14.2.2
npm install -D @testing-library/user-event@14.5.2
npm install -D eslint@8.57.0 prettier@3.2.5
```

- [ ] **Step 4: Create package.json scripts**

Edit `frontend/package.json`, add:

```json
{
  "name": "tikman-frontend",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "test": "vitest",
    "test:ui": "vitest --ui",
    "lint": "eslint . --ext ts,tsx",
    "format": "prettier --write \"src/**/*.{ts,tsx,css}\""
  }
}
```

- [ ] **Step 5: Create TypeScript config**

Create `frontend/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"],
      "@/domain/*": ["src/domain/*"],
      "@/infrastructure/*": ["src/infrastructure/*"],
      "@/application/*": ["src/application/*"],
      "@/presentation/*": ["src/presentation/*"],
      "@/shared/*": ["src/shared/*"]
    }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 6: Create Vite config**

Create `frontend/vite.config.ts`:

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react-swc';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@/domain': path.resolve(__dirname, './src/domain'),
      '@/infrastructure': path.resolve(__dirname, './src/infrastructure'),
      '@/application': path.resolve(__dirname, './src/application'),
      '@/presentation': path.resolve(__dirname, './src/presentation'),
      '@/shared': path.resolve(__dirname, './src/shared'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
});
```

- [ ] **Step 7: Create environment files**

Create `frontend/.env.example`:

```bash
VITE_API_URL=http://localhost:8080
VITE_APP_NAME=TikMan OLT Provisioning
```

Create `frontend/.env.development`:

```bash
VITE_API_URL=http://localhost:8080
VITE_APP_NAME=TikMan OLT Provisioning
```

- [ ] **Step 8: Create .gitignore**

Create `frontend/.gitignore`:

```
# Dependencies
node_modules

# Build output
dist
*.local

# Env files
.env.production

# IDE
.vscode
.idea
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Test
coverage
```

- [ ] **Step 9: Create HTML entry point**

Create `frontend/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>TikMan OLT Provisioning</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 10: Create minimal React app**

Create `frontend/src/main.tsx`:

```typescript
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

Create `frontend/src/App.tsx`:

```typescript
export default function App() {
  return (
    <div>
      <h1>TikMan OLT Provisioning</h1>
      <p>Frontend scaffolding complete</p>
    </div>
  );
}
```

Create `frontend/src/index.css`:

```css
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}

#root {
  min-height: 100vh;
}
```

Create `frontend/src/vite-env.d.ts`:

```typescript
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string;
  readonly VITE_APP_NAME: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
```

- [ ] **Step 11: Test dev server**

```bash
cd frontend
npm run dev
```

Expected: Browser opens at http://localhost:3000 showing "TikMan OLT Provisioning"

- [ ] **Step 12: Commit**

```bash
git add frontend/
git commit -m "feat(frontend): initialize React + TypeScript project with Vite

- Setup package.json with all dependencies
- Configure TypeScript with strict mode and path aliases
- Configure Vite with React SWC and proxy to backend
- Add environment variable configuration
- Create minimal React app entry point"
```

---

### Task 2: Domain Layer - Entities & Repository Interfaces

**Files:**
- Create: `frontend/src/domain/entities/User.ts`
- Create: `frontend/src/domain/entities/Site.ts`
- Create: `frontend/src/domain/entities/Olt.ts`
- Create: `frontend/src/domain/entities/index.ts`
- Create: `frontend/src/domain/repositories/IAuthRepository.ts`
- Create: `frontend/src/domain/repositories/IUserRepository.ts`
- Create: `frontend/src/domain/repositories/ISiteRepository.ts`
- Create: `frontend/src/domain/repositories/IOltRepository.ts`
- Create: `frontend/src/domain/repositories/index.ts`
- Test: `frontend/src/domain/__tests__/entities.test.ts`

**Interfaces:**
- Consumes: None (pure TypeScript)
- Produces:
  - `User` interface + `UserRole` enum
  - `Site` interface + DTOs
  - `Olt` interface + `OltProtocol`, `OltStatus` enums + DTOs
  - Repository interfaces for Auth, User, Site, OLT

- [ ] **Step 1: Write test for User entity**

Create `frontend/src/domain/__tests__/entities.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { UserRole, type User } from '../entities/User';

describe('User Entity', () => {
  it('should have correct UserRole enum values', () => {
    expect(UserRole.ADMIN).toBe('admin');
    expect(UserRole.TECHNICIAN).toBe('technician');
    expect(UserRole.VIEWER).toBe('viewer');
  });

  it('should create valid User object', () => {
    const user: User = {
      id: '123',
      username: 'admin',
      email: 'admin@test.com',
      role: UserRole.ADMIN,
      createdAt: '2024-01-01T00:00:00Z',
      updatedAt: '2024-01-01T00:00:00Z',
    };

    expect(user.username).toBe('admin');
    expect(user.role).toBe(UserRole.ADMIN);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
npm test
```

Expected: FAIL with "Cannot find module '../entities/User'"

- [ ] **Step 3: Create User entity**

Create `frontend/src/domain/entities/User.ts`:

```typescript
export enum UserRole {
  ADMIN = 'admin',
  TECHNICIAN = 'technician',
  VIEWER = 'viewer',
}

export interface User {
  id: string;
  username: string;
  email: string;
  role: UserRole;
  createdAt: string;
  updatedAt: string;
}

export interface CreateUserDto {
  username: string;
  email: string;
  password: string;
  role: UserRole;
}

export interface UpdateUserDto {
  username?: string;
  email?: string;
  password?: string;
  role?: UserRole;
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
npm test
```

Expected: PASS

- [ ] **Step 5: Create Site entity**

Create `frontend/src/domain/entities/Site.ts`:

```typescript
export interface Site {
  id: string;
  name: string;
  location: string;
  description: string;
  oltCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSiteDto {
  name: string;
  location?: string;
  description?: string;
}

export interface UpdateSiteDto {
  name?: string;
  location?: string;
  description?: string;
}
```

- [ ] **Step 6: Create OLT entity**

Create `frontend/src/domain/entities/Olt.ts`:

```typescript
export enum OltProtocol {
  SSH = 'ssh',
  TELNET = 'telnet',
}

export enum OltStatus {
  ONLINE = 'online',
  OFFLINE = 'offline',
  ERROR = 'error',
}

export interface Olt {
  id: string;
  siteId: string;
  siteName: string;
  name: string;
  ipAddress: string;
  protocol: OltProtocol;
  username: string;
  snmpCommunity: string;
  sshPort: number;
  telnetPort: number;
  snmpPort: number;
  status: OltStatus;
  lastSeen: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface CreateOltDto {
  siteId: string;
  name: string;
  ipAddress: string;
  protocol: OltProtocol;
  username: string;
  password: string;
  snmpCommunity?: string;
  sshPort?: number;
  telnetPort?: number;
  snmpPort?: number;
}

export interface UpdateOltDto {
  siteId?: string;
  name?: string;
  ipAddress?: string;
  protocol?: OltProtocol;
  username?: string;
  password?: string;
  snmpCommunity?: string;
  sshPort?: number;
  telnetPort?: number;
  snmpPort?: number;
}
```

- [ ] **Step 7: Create entities barrel export**

Create `frontend/src/domain/entities/index.ts`:

```typescript
export * from './User';
export * from './Site';
export * from './Olt';
```

- [ ] **Step 8: Create Auth repository interface**

Create `frontend/src/domain/repositories/IAuthRepository.ts`:

```typescript
import type { User } from '../entities';

export interface LoginCredentials {
  username: string;
  password: string;
}

export interface LoginResponse {
  user: User;
  token: string;
}

export interface IAuthRepository {
  login(credentials: LoginCredentials): Promise<LoginResponse>;
  logout(): Promise<void>;
  getCurrentUser(): Promise<User>;
}
```

- [ ] **Step 9: Create User repository interface**

Create `frontend/src/domain/repositories/IUserRepository.ts`:

```typescript
import type { User, CreateUserDto, UpdateUserDto } from '../entities';

export interface IUserRepository {
  getAll(): Promise<User[]>;
  getById(id: string): Promise<User>;
  create(data: CreateUserDto): Promise<User>;
  update(id: string, data: UpdateUserDto): Promise<User>;
  delete(id: string): Promise<void>;
}
```

- [ ] **Step 10: Create Site repository interface**

Create `frontend/src/domain/repositories/ISiteRepository.ts`:

```typescript
import type { Site, CreateSiteDto, UpdateSiteDto } from '../entities';

export interface ISiteRepository {
  getAll(): Promise<Site[]>;
  getById(id: string): Promise<Site>;
  create(data: CreateSiteDto): Promise<Site>;
  update(id: string, data: UpdateSiteDto): Promise<Site>;
  delete(id: string): Promise<void>;
}
```

- [ ] **Step 11: Create OLT repository interface**

Create `frontend/src/domain/repositories/IOltRepository.ts`:

```typescript
import type { Olt, CreateOltDto, UpdateOltDto } from '../entities';

export interface IOltRepository {
  getAll(): Promise<Olt[]>;
  getBySite(siteId: string): Promise<Olt[]>;
  getById(id: string): Promise<Olt>;
  create(data: CreateOltDto): Promise<Olt>;
  update(id: string, data: UpdateOltDto): Promise<Olt>;
  delete(id: string): Promise<void>;
}
```

- [ ] **Step 12: Create repositories barrel export**

Create `frontend/src/domain/repositories/index.ts`:

```typescript
export * from './IAuthRepository';
export * from './IUserRepository';
export * from './ISiteRepository';
export * from './IOltRepository';
```

- [ ] **Step 13: Run all tests**

```bash
npm test
```

Expected: All tests PASS

- [ ] **Step 14: Commit**

```bash
git add frontend/src/domain/
git commit -m "feat(frontend): add domain layer entities and repository interfaces

- Add User entity with UserRole enum and DTOs
- Add Site entity with DTOs
- Add OLT entity with protocol/status enums and DTOs
- Add repository interfaces for Auth, User, Site, OLT
- Pure TypeScript, no external dependencies
- Tests for entity types"
```

---

### Task 3: Infrastructure Layer - API Client & Error Handling

**Files:**
- Create: `frontend/src/shared/config/env.ts`
- Create: `frontend/src/infrastructure/http/apiClient.ts`
- Create: `frontend/src/infrastructure/http/endpoints.ts`
- Create: `frontend/src/infrastructure/http/errorMapper.ts`
- Create: `frontend/src/infrastructure/http/index.ts`
- Test: `frontend/src/infrastructure/__tests__/errorMapper.test.ts`

**Interfaces:**
- Consumes: `env.VITE_API_URL` from environment
- Produces:
  - `apiClient` - Axios instance configured with baseURL, withCredentials, interceptors
  - `API_ENDPOINTS` - Constant object with all API routes
  - `ApiError`, `ValidationError`, `UnauthorizedError`, `NotFoundError` classes
  - `mapApiError(error: AxiosError): ApiError` function

- [ ] **Step 1: Create environment config**

Create `frontend/src/shared/config/env.ts`:

```typescript
export const env = {
  apiUrl: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  appName: import.meta.env.VITE_APP_NAME || 'TikMan',
  isDevelopment: import.meta.env.DEV,
  isProduction: import.meta.env.PROD,
} as const;
```

- [ ] **Step 2: Write test for error mapper**

Create `frontend/src/infrastructure/__tests__/errorMapper.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { AxiosError } from 'axios';
import {
  ApiError,
  ValidationError,
  UnauthorizedError,
  NotFoundError,
  mapApiError,
} from '../http/errorMapper';

describe('Error Mapper', () => {
  it('should map 401 to UnauthorizedError', () => {
    const axiosError = {
      response: { status: 401, data: {} },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(UnauthorizedError);
    expect(result.statusCode).toBe(401);
    expect(result.code).toBe('UNAUTHORIZED');
  });

  it('should map 404 to NotFoundError', () => {
    const axiosError = {
      response: { status: 404, data: { resource: 'User' } },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(NotFoundError);
    expect(result.statusCode).toBe(404);
  });

  it('should map 400 with details to ValidationError', () => {
    const axiosError = {
      response: {
        status: 400,
        data: { code: 'VALIDATION_ERROR', details: { email: 'Invalid email' } },
      },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(ValidationError);
    expect(result.statusCode).toBe(400);
    expect((result as ValidationError).fields).toEqual({ email: 'Invalid email' });
  });

  it('should map unknown errors to generic ApiError', () => {
    const axiosError = {
      response: { status: 500, data: { code: 'SERVER_ERROR' } },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(ApiError);
    expect(result.statusCode).toBe(500);
    expect(result.code).toBe('SERVER_ERROR');
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

```bash
npm test
```

Expected: FAIL with "Cannot find module '../http/errorMapper'"

- [ ] **Step 4: Create error classes and mapper**

Create `frontend/src/infrastructure/http/errorMapper.ts`:

```typescript
import type { AxiosError } from 'axios';

export class ApiError extends Error {
  constructor(
    public statusCode: number,
    public code: string,
    public details?: Record<string, any>
  ) {
    super();
    this.name = 'ApiError';
    this.message = code;
  }
}

export class ValidationError extends ApiError {
  constructor(public fields: Record<string, string>) {
    super(400, 'VALIDATION_ERROR', fields);
    this.name = 'ValidationError';
  }
}

export class UnauthorizedError extends ApiError {
  constructor() {
    super(401, 'UNAUTHORIZED');
    this.name = 'UnauthorizedError';
    this.message = 'Session expired or invalid';
  }
}

export class NotFoundError extends ApiError {
  constructor(resource: string) {
    super(404, 'NOT_FOUND', { resource });
    this.name = 'NotFoundError';
    this.message = `${resource} not found`;
  }
}

export function mapApiError(error: AxiosError): ApiError {
  const response = error.response?.data as any;

  if (error.response?.status === 401) {
    return new UnauthorizedError();
  }

  if (error.response?.status === 404) {
    return new NotFoundError(response?.resource || 'Resource');
  }

  if (error.response?.status === 400 && response?.details) {
    return new ValidationError(response.details);
  }

  return new ApiError(
    error.response?.status || 500,
    response?.code || 'UNKNOWN_ERROR',
    response?.details
  );
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
npm test
```

Expected: PASS

- [ ] **Step 6: Create API endpoints constants**

Create `frontend/src/infrastructure/http/endpoints.ts`:

```typescript
export const API_ENDPOINTS = {
  // Auth
  AUTH_LOGIN: '/api/v1/auth/login',
  AUTH_LOGOUT: '/api/v1/auth/logout',
  AUTH_ME: '/api/v1/auth/me',

  // Users
  USERS: '/api/v1/users',
  USER_BY_ID: (id: string) => `/api/v1/users/${id}`,

  // Sites
  SITES: '/api/v1/sites',
  SITE_BY_ID: (id: string) => `/api/v1/sites/${id}`,

  // OLTs
  OLTS: '/api/v1/olts',
  OLT_BY_ID: (id: string) => `/api/v1/olts/${id}`,
} as const;
```

- [ ] **Step 7: Create API client**

Create `frontend/src/infrastructure/http/apiClient.ts`:

```typescript
import axios from 'axios';
import { env } from '@/shared/config/env';
import { mapApiError } from './errorMapper';

export const apiClient = axios.create({
  baseURL: env.apiUrl,
  withCredentials: true, // Important: send cookies
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor
apiClient.interceptors.request.use(
  (config) => {
    // Add correlation ID for logging
    config.headers['X-Request-ID'] = crypto.randomUUID();
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const mappedError = mapApiError(error);

    // Auto logout on 401 will be handled by auth store
    // (imported dynamically to avoid circular dependency)
    if (error.response?.status === 401) {
      // Store handles logout
    }

    return Promise.reject(mappedError);
  }
);
```

- [ ] **Step 8: Create barrel export**

Create `frontend/src/infrastructure/http/index.ts`:

```typescript
export * from './apiClient';
export * from './endpoints';
export * from './errorMapper';
```

- [ ] **Step 9: Run all tests**

```bash
npm test
```

Expected: All tests PASS

- [ ] **Step 10: Commit**

```bash
git add frontend/src/infrastructure/ frontend/src/shared/
git commit -m "feat(frontend): add infrastructure layer with API client

- Add environment configuration helper
- Add API error classes and mapper
- Add API endpoints constants
- Configure Axios client with interceptors
- Add withCredentials for cookie auth
- Add request correlation IDs
- Tests for error mapping"
```

---

### Task 4: Infrastructure Layer - Repository Implementations

**Files:**
- Create: `frontend/src/infrastructure/repositories/AuthRepository.ts`
- Create: `frontend/src/infrastructure/repositories/UserRepository.ts`
- Create: `frontend/src/infrastructure/repositories/SiteRepository.ts`
- Create: `frontend/src/infrastructure/repositories/OltRepository.ts`
- Create: `frontend/src/infrastructure/repositories/index.ts`
- Test: `frontend/src/infrastructure/__tests__/repositories.test.ts`

**Interfaces:**
- Consumes:
  - `IAuthRepository`, `IUserRepository`, `ISiteRepository`, `IOltRepository` from domain
  - `apiClient` from infrastructure/http
  - `API_ENDPOINTS` from infrastructure/http
- Produces:
  - `AuthRepository` implements `IAuthRepository`
  - `UserRepository` implements `IUserRepository`
  - `SiteRepository` implements `ISiteRepository`
  - `OltRepository` implements `IOltRepository`

- [ ] **Step 1: Write test for UserRepository**

Create `frontend/src/infrastructure/__tests__/repositories.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { UserRepository } from '../repositories/UserRepository';
import type { CreateUserDto } from '@/domain/entities';
import { UserRole } from '@/domain/entities';

describe('UserRepository', () => {
  let mockClient: any;
  let repository: UserRepository;

  beforeEach(() => {
    mockClient = {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
    };
    // @ts-ignore - testing with mock
    repository = new UserRepository();
    // @ts-ignore - inject mock
    repository.client = mockClient;
  });

  it('should fetch all users', async () => {
    const mockUsers = [{ id: '1', username: 'admin', role: UserRole.ADMIN }];
    mockClient.get.mockResolvedValue({ data: mockUsers });

    const users = await repository.getAll();

    expect(users).toEqual(mockUsers);
    expect(mockClient.get).toHaveBeenCalledWith('/api/v1/users');
  });

  it('should create user', async () => {
    const createDto: CreateUserDto = {
      username: 'test',
      email: 'test@test.com',
      password: 'password123',
      role: UserRole.TECHNICIAN,
    };
    const mockUser = { id: '2', ...createDto };
    mockClient.post.mockResolvedValue({ data: mockUser });

    const user = await repository.create(createDto);

    expect(user).toEqual(mockUser);
    expect(mockClient.post).toHaveBeenCalledWith('/api/v1/users', createDto);
  });

  it('should delete user', async () => {
    mockClient.delete.mockResolvedValue({});

    await repository.delete('123');

    expect(mockClient.delete).toHaveBeenCalledWith('/api/v1/users/123');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
npm test
```

Expected: FAIL with "Cannot find module '../repositories/UserRepository'"

- [ ] **Step 3: Create AuthRepository**

Create `frontend/src/infrastructure/repositories/AuthRepository.ts`:

```typescript
import { apiClient } from '../http/apiClient';
import { API_ENDPOINTS } from '../http/endpoints';
import type {
  IAuthRepository,
  LoginCredentials,
  LoginResponse,
} from '@/domain/repositories';
import type { User } from '@/domain/entities';

export class AuthRepository implements IAuthRepository {
  async login(credentials: LoginCredentials): Promise<LoginResponse> {
    const response = await apiClient.post(API_ENDPOINTS.AUTH_LOGIN, credentials);
    return response.data;
  }

  async logout(): Promise<void> {
    await apiClient.post(API_ENDPOINTS.AUTH_LOGOUT);
  }

  async getCurrentUser(): Promise<User> {
    const response = await apiClient.get(API_ENDPOINTS.AUTH_ME);
    return response.data;
  }
}
```

- [ ] **Step 4: Create UserRepository**

Create `frontend/src/infrastructure/repositories/UserRepository.ts`:

```typescript
import { apiClient } from '../http/apiClient';
import { API_ENDPOINTS } from '../http/endpoints';
import type { IUserRepository } from '@/domain/repositories';
import type { User, CreateUserDto, UpdateUserDto } from '@/domain/entities';

export class UserRepository implements IUserRepository {
  private client = apiClient;

  async getAll(): Promise<User[]> {
    const response = await this.client.get(API_ENDPOINTS.USERS);
    return response.data;
  }

  async getById(id: string): Promise<User> {
    const response = await this.client.get(API_ENDPOINTS.USER_BY_ID(id));
    return response.data;
  }

  async create(data: CreateUserDto): Promise<User> {
    const response = await this.client.post(API_ENDPOINTS.USERS, data);
    return response.data;
  }

  async update(id: string, data: UpdateUserDto): Promise<User> {
    const response = await this.client.put(API_ENDPOINTS.USER_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await this.client.delete(API_ENDPOINTS.USER_BY_ID(id));
  }
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
npm test
```

Expected: PASS

- [ ] **Step 6: Create SiteRepository**

Create `frontend/src/infrastructure/repositories/SiteRepository.ts`:

```typescript
import { apiClient } from '../http/apiClient';
import { API_ENDPOINTS } from '../http/endpoints';
import type { ISiteRepository } from '@/domain/repositories';
import type { Site, CreateSiteDto, UpdateSiteDto } from '@/domain/entities';

export class SiteRepository implements ISiteRepository {
  async getAll(): Promise<Site[]> {
    const response = await apiClient.get(API_ENDPOINTS.SITES);
    return response.data;
  }

  async getById(id: string): Promise<Site> {
    const response = await apiClient.get(API_ENDPOINTS.SITE_BY_ID(id));
    return response.data;
  }

  async create(data: CreateSiteDto): Promise<Site> {
    const response = await apiClient.post(API_ENDPOINTS.SITES, data);
    return response.data;
  }

  async update(id: string, data: UpdateSiteDto): Promise<Site> {
    const response = await apiClient.put(API_ENDPOINTS.SITE_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.SITE_BY_ID(id));
  }
}
```

- [ ] **Step 7: Create OltRepository**

Create `frontend/src/infrastructure/repositories/OltRepository.ts`:

```typescript
import { apiClient } from '../http/apiClient';
import { API_ENDPOINTS } from '../http/endpoints';
import type { IOltRepository } from '@/domain/repositories';
import type { Olt, CreateOltDto, UpdateOltDto } from '@/domain/entities';

export class OltRepository implements IOltRepository {
  async getAll(): Promise<Olt[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLTS);
    return response.data;
  }

  async getBySite(siteId: string): Promise<Olt[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLTS, {
      params: { site_id: siteId },
    });
    return response.data;
  }

  async getById(id: string): Promise<Olt> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_BY_ID(id));
    return response.data;
  }

  async create(data: CreateOltDto): Promise<Olt> {
    const response = await apiClient.post(API_ENDPOINTS.OLTS, data);
    return response.data;
  }

  async update(id: string, data: UpdateOltDto): Promise<Olt> {
    const response = await apiClient.put(API_ENDPOINTS.OLT_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.OLT_BY_ID(id));
  }
}
```

- [ ] **Step 8: Create barrel export**

Create `frontend/src/infrastructure/repositories/index.ts`:

```typescript
export * from './AuthRepository';
export * from './UserRepository';
export * from './SiteRepository';
export * from './OltRepository';
```

- [ ] **Step 9: Run all tests**

```bash
npm test
```

Expected: All tests PASS

- [ ] **Step 10: Commit**

```bash
git add frontend/src/infrastructure/repositories/
git commit -m "feat(frontend): add repository implementations

- Implement AuthRepository for login/logout/getCurrentUser
- Implement UserRepository for CRUD operations
- Implement SiteRepository for CRUD operations
- Implement OltRepository for CRUD operations
- All implement domain repository interfaces
- Tests for repository methods"
```

---

### Task 5: Application Layer - Auth Store (Zustand)

**Files:**
- Create: `frontend/src/application/stores/authStore.ts`
- Create: `frontend/src/application/stores/index.ts`
- Test: `frontend/src/application/__tests__/authStore.test.ts`

**Interfaces:**
- Consumes:
  - `AuthRepository` from infrastructure
  - `User` type from domain
- Produces:
  - `useAuthStore()` hook with: `user`, `isAuthenticated`, `login()`, `logout()`, `checkAuth()`
  - Auth state persisted in memory only (no localStorage for security)

- [ ] **Step 1: Write test for auth store**

Create `frontend/src/application/__tests__/authStore.test.ts`:

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAuthStore } from '../stores/authStore';

describe('Auth Store', () => {
  beforeEach(() => {
    const { result } = renderHook(() => useAuthStore());
    act(() => {
      result.current.logout();
    });
  });

  it('should initialize with unauthenticated state', () => {
    const { result } = renderHook(() => useAuthStore());

    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.user).toBeNull();
  });

  it('should set user on successful login', async () => {
    const { result } = renderHook(() => useAuthStore());
    const mockUser = { id: '1', username: 'admin', role: 'admin' };

    act(() => {
      result.current.setUser(mockUser as any);
    });

    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.user).toEqual(mockUser);
  });

  it('should clear user on logout', async () => {
    const { result } = renderHook(() => useAuthStore());
    const mockUser = { id: '1', username: 'admin', role: 'admin' };

    act(() => {
      result.current.setUser(mockUser as any);
    });
    expect(result.current.isAuthenticated).toBe(true);

    act(() => {
      result.current.logout();
    });

    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.user).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
npm test
```

Expected: FAIL with "Cannot find module '../stores/authStore'"

- [ ] **Step 3: Create auth store**

Create `frontend/src/application/stores/authStore.ts`:

```typescript
import { create } from 'zustand';
import type { User } from '@/domain/entities';

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  setUser: (user: User | null) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  
  setUser: (user) =>
    set({
      user,
      isAuthenticated: user !== null,
    }),

  logout: () =>
    set({
      user: null,
      isAuthenticated: false,
    }),
}));
```

- [ ] **Step 4: Run test to verify it passes**

```bash
npm test
```

Expected: PASS

- [ ] **Step 5: Create barrel export**

Create `frontend/src/application/stores/index.ts`:

```typescript
export * from './authStore';
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/application/
git commit -m "feat(frontend): add auth store with Zustand

- Implement auth state management
- Store user and authentication status
- Provide login/logout actions
- Memory-only storage for security
- Tests for auth store state"
```

---

### Task 6: Application Layer - React Query Hooks

**Files:**
- Create: `frontend/src/application/hooks/useAuth.ts`
- Create: `frontend/src/application/hooks/useUsers.ts`
- Create: `frontend/src/application/hooks/useSites.ts`
- Create: `frontend/src/application/hooks/useOlts.ts`
- Create: `frontend/src/application/hooks/index.ts`
- Create: `frontend/src/shared/config/queryClient.ts`
- Test: `frontend/src/application/__tests__/hooks.test.ts`

**Interfaces:**
- Consumes:
  - Repository instances (AuthRepository, UserRepository, SiteRepository, OltRepository)
  - `useAuthStore()` from application/stores
- Produces:
  - `useLogin()`, `useLogout()`, `useCurrentUser()` - auth mutations/queries
  - `useUsers()`, `useCreateUser()`, `useUpdateUser()`, `useDeleteUser()` - user CRUD
  - `useSites()`, `useCreateSite()`, `useUpdateSite()`, `useDeleteSite()` - site CRUD
  - `useOlts()`, `useCreateOlt()`, `useUpdateOlt()`, `useDeleteOlt()` - OLT CRUD

- [ ] **Step 1: Create query client config**

Create `frontend/src/shared/config/queryClient.ts`:

```typescript
import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5,
      refetchOnWindowFocus: false,
      retry: 1,
    },
    mutations: {
      retry: false,
    },
  },
});
```

- [ ] **Step 2: Write test for auth hooks**

Create `frontend/src/application/__tests__/hooks.test.ts`:

```typescript
import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useLogin } from '../hooks/useAuth';

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('useLogin hook', () => {
  it('should call login mutation', async () => {
    const { result } = renderHook(() => useLogin(), { wrapper: createWrapper() });

    expect(result.current.isPending).toBe(false);
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

```bash
npm test
```

Expected: FAIL with "Cannot find module '../hooks/useAuth'"

- [ ] **Step 4: Create auth hooks**

Create `frontend/src/application/hooks/useAuth.ts`:

```typescript
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AuthRepository } from '@/infrastructure/repositories';
import { useAuthStore } from '../stores';
import type { LoginCredentials } from '@/domain/repositories';

const authRepository = new AuthRepository();

export function useLogin() {
  const queryClient = useQueryClient();
  const setUser = useAuthStore((state) => state.setUser);

  return useMutation({
    mutationFn: (credentials: LoginCredentials) => authRepository.login(credentials),
    onSuccess: (data) => {
      setUser(data.user);
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] });
    },
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  const logout = useAuthStore((state) => state.logout);

  return useMutation({
    mutationFn: () => authRepository.logout(),
    onSuccess: () => {
      logout();
      queryClient.clear();
    },
  });
}

export function useCurrentUser() {
  const setUser = useAuthStore((state) => state.setUser);

  return useQuery({
    queryKey: ['auth', 'me'],
    queryFn: async () => {
      const user = await authRepository.getCurrentUser();
      setUser(user);
      return user;
    },
    retry: false,
  });
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
npm test
```

Expected: PASS

- [ ] **Step 6: Create user hooks**

Create `frontend/src/application/hooks/useUsers.ts`:

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { UserRepository } from '@/infrastructure/repositories';
import type { CreateUserDto, UpdateUserDto } from '@/domain/entities';

const userRepository = new UserRepository();

export function useUsers() {
  return useQuery({
    queryKey: ['users'],
    queryFn: () => userRepository.getAll(),
  });
}

export function useUser(id: string) {
  return useQuery({
    queryKey: ['users', id],
    queryFn: () => userRepository.getById(id),
    enabled: !!id,
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateUserDto) => userRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
    },
  });
}

export function useUpdateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateUserDto }) =>
      userRepository.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      queryClient.invalidateQueries({ queryKey: ['users', variables.id] });
    },
  });
}

export function useDeleteUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => userRepository.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
    },
  });
}
```

- [ ] **Step 7: Create site hooks**

Create `frontend/src/application/hooks/useSites.ts`:

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { SiteRepository } from '@/infrastructure/repositories';
import type { CreateSiteDto, UpdateSiteDto } from '@/domain/entities';

const siteRepository = new SiteRepository();

export function useSites() {
  return useQuery({
    queryKey: ['sites'],
    queryFn: () => siteRepository.getAll(),
  });
}

export function useSite(id: string) {
  return useQuery({
    queryKey: ['sites', id],
    queryFn: () => siteRepository.getById(id),
    enabled: !!id,
  });
}

export function useCreateSite() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateSiteDto) => siteRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sites'] });
    },
  });
}

export function useUpdateSite() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateSiteDto }) =>
      siteRepository.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['sites'] });
      queryClient.invalidateQueries({ queryKey: ['sites', variables.id] });
    },
  });
}

export function useDeleteSite() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => siteRepository.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sites'] });
    },
  });
}
```

- [ ] **Step 8: Create OLT hooks**

Create `frontend/src/application/hooks/useOlts.ts`:

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { OltRepository } from '@/infrastructure/repositories';
import type { CreateOltDto, UpdateOltDto } from '@/domain/entities';

const oltRepository = new OltRepository();

export function useOlts(siteId?: string) {
  return useQuery({
    queryKey: siteId ? ['olts', 'site', siteId] : ['olts'],
    queryFn: () => (siteId ? oltRepository.getBySite(siteId) : oltRepository.getAll()),
  });
}

export function useOlt(id: string) {
  return useQuery({
    queryKey: ['olts', id],
    queryFn: () => oltRepository.getById(id),
    enabled: !!id,
  });
}

export function useCreateOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateOltDto) => oltRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['olts'] });
      queryClient.invalidateQueries({ queryKey: ['sites'] });
    },
  });
}

export function useUpdateOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateOltDto }) =>
      oltRepository.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['olts'] });
      queryClient.invalidateQueries({ queryKey: ['olts', variables.id] });
    },
  });
}

export function useDeleteOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => oltRepository.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['olts'] });
      queryClient.invalidateQueries({ queryKey: ['sites'] });
    },
  });
}
```

- [ ] **Step 9: Create barrel export**

Create `frontend/src/application/hooks/index.ts`:

```typescript
export * from './useAuth';
export * from './useUsers';
export * from './useSites';
export * from './useOlts';
```

- [ ] **Step 10: Run all tests**

```bash
npm test
```

Expected: All tests PASS

- [ ] **Step 11: Commit**

```bash
git add frontend/src/application/ frontend/src/shared/
git commit -m "feat(frontend): add React Query hooks for data management

- Add query client configuration
- Add auth hooks (login, logout, getCurrentUser)
- Add user CRUD hooks
- Add site CRUD hooks
- Add OLT CRUD hooks
- Auto-invalidate queries on mutations
- Tests for hooks"
```

---

### Task 7: Presentation Layer - Router Setup & Protected Routes

**Files:**
- Create: `frontend/src/presentation/routes/index.tsx`
- Create: `frontend/src/presentation/routes/ProtectedRoute.tsx`
- Create: `frontend/src/presentation/pages/Login.tsx`
- Create: `frontend/src/presentation/pages/Dashboard.tsx`
- Create: `frontend/src/presentation/pages/NotFound.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/main.tsx`

**Interfaces:**
- Consumes:
  - `useAuthStore()` from application
  - `useCurrentUser()` from application
- Produces:
  - Router with routes: `/login`, `/`, `/users`, `/sites`, `/olts`
  - `ProtectedRoute` component that redirects to login if not authenticated
  - Placeholder pages for all routes

- [ ] **Step 1: Create ProtectedRoute component**

Create `frontend/src/presentation/routes/ProtectedRoute.tsx`:

```typescript
import { Navigate, Outlet } from 'react-router-dom';
import { useAuthStore } from '@/application/stores';
import { Spin } from 'antd';
import { useCurrentUser } from '@/application/hooks';

export function ProtectedRoute() {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const { isLoading } = useCurrentUser();

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  return isAuthenticated ? <Outlet /> : <Navigate to="/login" replace />;
}
```

- [ ] **Step 2: Create placeholder Login page**

Create `frontend/src/presentation/pages/Login.tsx`:

```typescript
export default function LoginPage() {
  return (
    <div>
      <h1>Login Page</h1>
      <p>Login form will be implemented in next task</p>
    </div>
  );
}
```

- [ ] **Step 3: Create placeholder Dashboard page**

Create `frontend/src/presentation/pages/Dashboard.tsx`:

```typescript
export default function DashboardPage() {
  return (
    <div>
      <h1>Dashboard</h1>
      <p>Dashboard content will be implemented in next task</p>
    </div>
  );
}
```

- [ ] **Step 4: Create NotFound page**

Create `frontend/src/presentation/pages/NotFound.tsx`:

```typescript
import { Result, Button } from 'antd';
import { useNavigate } from 'react-router-dom';

export default function NotFoundPage() {
  const navigate = useNavigate();

  return (
    <Result
      status="404"
      title="404"
      subTitle="Halaman tidak ditemukan"
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          Kembali ke Dashboard
        </Button>
      }
    />
  );
}
```

- [ ] **Step 5: Create router configuration**

Create `frontend/src/presentation/routes/index.tsx`:

```typescript
import { createBrowserRouter, Navigate } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import LoginPage from '../pages/Login';
import DashboardPage from '../pages/Dashboard';
import NotFoundPage from '../pages/NotFound';

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    element: <ProtectedRoute />,
    children: [
      {
        index: true,
        element: <DashboardPage />,
      },
      {
        path: 'users',
        element: <div>Users Page (placeholder)</div>,
      },
      {
        path: 'sites',
        element: <div>Sites Page (placeholder)</div>,
      },
      {
        path: 'olts',
        element: <div>OLTs Page (placeholder)</div>,
      },
    ],
  },
  {
    path: '/404',
    element: <NotFoundPage />,
  },
  {
    path: '*',
    element: <Navigate to="/404" replace />,
  },
]);
```

- [ ] **Step 6: Update App.tsx to use router**

Edit `frontend/src/App.tsx`:

```typescript
import { RouterProvider } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { ConfigProvider } from 'antd';
import { queryClient } from '@/shared/config/queryClient';
import { router } from './presentation/routes';

export default function App() {
  return (
    <ConfigProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ConfigProvider>
  );
}
```

- [ ] **Step 7: Update main.tsx**

Edit `frontend/src/main.tsx`:

```typescript
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

- [ ] **Step 8: Test router in browser**

```bash
npm run dev
```

Expected: 
- Navigate to http://localhost:3000 → should redirect to /login
- Login page shows placeholder
- Direct access to /users should redirect to /login

- [ ] **Step 9: Commit**

```bash
git add frontend/src/
git commit -m "feat(frontend): add router and protected routes

- Setup React Router v6
- Add ProtectedRoute component with auth check
- Add placeholder pages (Login, Dashboard, NotFound)
- Configure routes with authentication guard
- Integrate QueryClient and ConfigProvider in App"
```# TikMan Frontend - Part 2 (Tasks 8-15)

> This is a continuation of the main plan. Tasks 1-7 are in `2026-08-14-tikman-frontend-implementation.md`

---

### Task 8: Presentation Layer - Layout Components

**Files:**
- Create: `frontend/src/presentation/components/layout/AppLayout.tsx`
- Create: `frontend/src/presentation/components/layout/Sidebar.tsx`
- Create: `frontend/src/presentation/components/layout/Header.tsx`
- Create: `frontend/src/presentation/components/layout/index.ts`
- Modify: `frontend/src/presentation/routes/index.tsx`

**Interfaces:**
- Consumes:
  - `useAuthStore()` from application
  - `useLogout()` from application
  - Ant Design Layout, Menu, Avatar, Dropdown components
- Produces:
  - `AppLayout` component with sidebar + header
  - Navigation menu with role-based visibility
  - User profile dropdown with logout

- [ ] **Step 1: Create Header component**

Create `frontend/src/presentation/components/layout/Header.tsx`:

```typescript
import { Layout, Dropdown, Avatar, Button, Space, Typography } from 'antd';
import { UserOutlined, LogoutOutlined } from '@ant-design/icons';
import { useAuthStore } from '@/application/stores';
import { useLogout } from '@/application/hooks';
import type { MenuProps } from 'antd';

const { Header: AntHeader } = Layout;
const { Text } = Typography;

export function Header() {
  const user = useAuthStore((state) => state.user);
  const logoutMutation = useLogout();

  const handleLogout = () => {
    logoutMutation.mutate();
  };

  const items: MenuProps['items'] = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: (
        <div>
          <Text strong>{user?.username}</Text>
          <br />
          <Text type="secondary" style={{ fontSize: '12px' }}>
            {user?.role}
          </Text>
        </div>
      ),
    },
    { type: 'divider' },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: 'Logout',
      onClick: handleLogout,
    },
  ];

  return (
    <AntHeader style={{ background: '#fff', padding: '0 24px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <Typography.Title level={4} style={{ margin: 0 }}>
        TikMan OLT Provisioning
      </Typography.Title>
      <Dropdown menu={{ items }} placement="bottomRight">
        <Space style={{ cursor: 'pointer' }}>
          <Avatar icon={<UserOutlined />} />
          <Text>{user?.username}</Text>
        </Space>
      </Dropdown>
    </AntHeader>
  );
}
```

- [ ] **Step 2: Create Sidebar component**

Create `frontend/src/presentation/components/layout/Sidebar.tsx`:

```typescript
import { Layout, Menu } from 'antd';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  DashboardOutlined,
  UserOutlined,
  EnvironmentOutlined,
  ApiOutlined,
} from '@ant-design/icons';
import { useAuthStore } from '@/application/stores';
import { UserRole } from '@/domain/entities';
import type { MenuProps } from 'antd';

const { Sider } = Layout;

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = useAuthStore((state) => state.user);

  const items: MenuProps['items'] = [
    {
      key: '/',
      icon: <DashboardOutlined />,
      label: 'Dashboard',
      onClick: () => navigate('/'),
    },
    {
      key: '/sites',
      icon: <EnvironmentOutlined />,
      label: 'Sites',
      onClick: () => navigate('/sites'),
    },
    {
      key: '/olts',
      icon: <ApiOutlined />,
      label: 'OLTs',
      onClick: () => navigate('/olts'),
    },
  ];

  if (user?.role === UserRole.ADMIN) {
    items.push({
      key: '/users',
      icon: <UserOutlined />,
      label: 'Users',
      onClick: () => navigate('/users'),
    });
  }

  const selectedKey = items.find((item) => location.pathname === item?.key)?.key as string || '/';

  return (
    <Sider width={250} style={{ background: '#001529' }}>
      <div style={{ height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '18px', fontWeight: 'bold' }}>
        TikMan
      </div>
      <Menu
        mode="inline"
        selectedKeys={[selectedKey]}
        items={items}
        theme="dark"
      />
    </Sider>
  );
}
```

- [ ] **Step 3: Create AppLayout component**

Create `frontend/src/presentation/components/layout/AppLayout.tsx`:

```typescript
import { Layout } from 'antd';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';

const { Content } = Layout;

export function AppLayout() {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar />
      <Layout>
        <Header />
        <Content style={{ margin: '24px', background: '#fff', padding: 24 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
```

- [ ] **Step 4: Create barrel export**

Create `frontend/src/presentation/components/layout/index.ts`:

```typescript
export * from './AppLayout';
export * from './Sidebar';
export * from './Header';
```

- [ ] **Step 5: Update routes to use AppLayout**

Edit `frontend/src/presentation/routes/index.tsx`:

```typescript
import { createBrowserRouter, Navigate } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import { AppLayout } from '../components/layout';
import LoginPage from '../pages/Login';
import DashboardPage from '../pages/Dashboard';
import NotFoundPage from '../pages/NotFound';

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          {
            index: true,
            element: <DashboardPage />,
          },
          {
            path: 'users',
            element: <div>Users Page (placeholder)</div>,
          },
          {
            path: 'sites',
            element: <div>Sites Page (placeholder)</div>,
          },
          {
            path: 'olts',
            element: <div>OLTs Page (placeholder)</div>,
          },
        ],
      },
    ],
  },
  {
    path: '/404',
    element: <NotFoundPage />,
  },
  {
    path: '*',
    element: <Navigate to="/404" replace />,
  },
]);
```

- [ ] **Step 6: Test layout in browser**

```bash
npm run dev
```

Expected: After login (placeholder), should see sidebar with menu and header with user info

- [ ] **Step 7: Commit**

```bash
git add frontend/src/presentation/
git commit -m "feat(frontend): add application layout components

- Add AppLayout with sidebar and header
- Add Sidebar with navigation menu
- Add Header with user dropdown and logout
- Role-based menu visibility (Admin sees Users)
- Integrate layout into protected routes"
```

---

### Task 9: Presentation Layer - Login Page

**Files:**
- Modify: `frontend/src/presentation/pages/Login.tsx`
- Create: `frontend/src/presentation/pages/Login.module.css`

**Interfaces:**
- Consumes:
  - `useLogin()` from application
  - Ant Design Form, Input, Button, Card, Alert components
- Produces:
  - Full login page with form
  - Error handling for failed login
  - Redirect to dashboard on success

- [ ] **Step 1: Create Login page**

Edit `frontend/src/presentation/pages/Login.tsx`:

```typescript
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Form, Input, Button, Card, Typography, Alert } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { useLogin } from '@/application/hooks';
import { useAuthStore } from '@/application/stores';
import type { LoginCredentials } from '@/domain/repositories';

const { Title, Text } = Typography;

export default function LoginPage() {
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const loginMutation = useLogin();

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/', { replace: true });
    }
  }, [isAuthenticated, navigate]);

  const handleSubmit = (values: LoginCredentials) => {
    loginMutation.mutate(values, {
      onSuccess: () => {
        navigate('/', { replace: true });
      },
    });
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
      <Card style={{ width: 400, boxShadow: '0 2px 8px rgba(0,0,0,0.1)' }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <Title level={2}>TikMan</Title>
          <Text type="secondary">ZTE OLT Provisioning System</Text>
        </div>

        {loginMutation.isError && (
          <Alert
            message="Login Gagal"
            description="Username atau password salah"
            type="error"
            showIcon
            closable
            style={{ marginBottom: 16 }}
          />
        )}

        <Form
          name="login"
          onFinish={handleSubmit}
          autoComplete="off"
          layout="vertical"
        >
          <Form.Item
            name="username"
            rules={[{ required: true, message: 'Username harus diisi' }]}
          >
            <Input
              prefix={<UserOutlined />}
              placeholder="Username"
              size="large"
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: 'Password harus diisi' }]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="Password"
              size="large"
            />
          </Form.Item>

          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              size="large"
              block
              loading={loginMutation.isPending}
            >
              Login
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
```

- [ ] **Step 2: Test login page**

```bash
npm run dev
```

Expected:
- Login form renders correctly
- Form validation works
- Submit button shows loading state
- Error alert shows on failed login

- [ ] **Step 3: Commit**

```bash
git add frontend/src/presentation/pages/
git commit -m "feat(frontend): implement login page

- Add login form with validation
- Add error handling for failed login
- Add loading state during login
- Auto-redirect to dashboard on success
- Responsive card-based layout"
```

---

### Task 10: Presentation Layer - Dashboard Page

**Files:**
- Modify: `frontend/src/presentation/pages/Dashboard.tsx`
- Create: `frontend/src/presentation/components/dashboard/StatsCard.tsx`
- Create: `frontend/src/presentation/components/dashboard/index.ts`

**Interfaces:**
- Consumes:
  - `useUsers()`, `useSites()`, `useOlts()` from application
  - Ant Design Card, Statistic, Row, Col components
- Produces:
  - Dashboard with statistics cards
  - Total users, sites, OLTs count
  - Loading states

- [ ] **Step 1: Create StatsCard component**

Create `frontend/src/presentation/components/dashboard/StatsCard.tsx`:

```typescript
import { Card, Statistic } from 'antd';
import type { ReactNode } from 'react';

interface StatsCardProps {
  title: string;
  value: number;
  icon: ReactNode;
  loading?: boolean;
}

export function StatsCard({ title, value, icon, loading }: StatsCardProps) {
  return (
    <Card>
      <Statistic
        title={title}
        value={value}
        prefix={icon}
        loading={loading}
      />
    </Card>
  );
}
```

- [ ] **Step 2: Create barrel export**

Create `frontend/src/presentation/components/dashboard/index.ts`:

```typescript
export * from './StatsCard';
```

- [ ] **Step 3: Implement Dashboard page**

Edit `frontend/src/presentation/pages/Dashboard.tsx`:

```typescript
import { Row, Col, Typography } from 'antd';
import { UserOutlined, EnvironmentOutlined, ApiOutlined } from '@ant-design/icons';
import { useUsers, useSites, useOlts } from '@/application/hooks';
import { StatsCard } from '../components/dashboard';
import { useAuthStore } from '@/application/stores';
import { UserRole } from '@/domain/entities';

const { Title } = Typography;

export default function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const { data: users, isLoading: usersLoading } = useUsers();
  const { data: sites, isLoading: sitesLoading } = useSites();
  const { data: olts, isLoading: oltsLoading } = useOlts();

  return (
    <div>
      <Title level={2}>Dashboard</Title>
      <Typography.Paragraph type="secondary">
        Selamat datang, {user?.username}!
      </Typography.Paragraph>

      <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
        {user?.role === UserRole.ADMIN && (
          <Col xs={24} sm={12} lg={8}>
            <StatsCard
              title="Total Users"
              value={users?.length || 0}
              icon={<UserOutlined />}
              loading={usersLoading}
            />
          </Col>
        )}
        <Col xs={24} sm={12} lg={8}>
          <StatsCard
            title="Total Sites"
            value={sites?.length || 0}
            icon={<EnvironmentOutlined />}
            loading={sitesLoading}
          />
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <StatsCard
            title="Total OLTs"
            value={olts?.length || 0}
            icon={<ApiOutlined />}
            loading={oltsLoading}
          />
        </Col>
      </Row>
    </div>
  );
}
```

- [ ] **Step 4: Test dashboard**

```bash
npm run dev
```

Expected:
- Dashboard shows stats cards
- User count only visible for Admin
- Loading states work correctly

- [ ] **Step 5: Commit**

```bash
git add frontend/src/presentation/
git commit -m "feat(frontend): implement dashboard page

- Add statistics cards for users, sites, OLTs
- Add role-based card visibility
- Add loading states for stats
- Responsive grid layout"
```

---

### Task 11: Presentation Layer - Users Management (CRUD)

**Files:**
- Create: `frontend/src/presentation/pages/Users.tsx`
- Create: `frontend/src/presentation/components/users/UserTable.tsx`
- Create: `frontend/src/presentation/components/users/UserModal.tsx`
- Create: `frontend/src/presentation/components/users/index.ts`
- Modify: `frontend/src/presentation/routes/index.tsx`

**Interfaces:**
- Consumes:
  - `useUsers()`, `useCreateUser()`, `useUpdateUser()`, `useDeleteUser()` from application
  - `User`, `CreateUserDto`, `UpdateUserDto`, `UserRole` from domain
- Produces:
  - Users list table with actions
  - Create/Edit user modal
  - Delete confirmation
  - Role selection

- [ ] **Step 1: Create UserModal component**

Create `frontend/src/presentation/components/users/UserModal.tsx`:

```typescript
import { Modal, Form, Input, Select } from 'antd';
import { UserRole, type User, type CreateUserDto, type UpdateUserDto } from '@/domain/entities';
import { useEffect } from 'react';

interface UserModalProps {
  open: boolean;
  user?: User;
  onClose: () => void;
  onSubmit: (data: CreateUserDto | UpdateUserDto) => void;
  loading: boolean;
}

export function UserModal({ open, user, onClose, onSubmit, loading }: UserModalProps) {
  const [form] = Form.useForm();

  useEffect(() => {
    if (user) {
      form.setFieldsValue({
        username: user.username,
        email: user.email,
        role: user.role,
      });
    } else {
      form.resetFields();
    }
  }, [user, form]);

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      onSubmit(values);
    });
  };

  return (
    <Modal
      title={user ? 'Edit User' : 'Create User'}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="username"
          label="Username"
          rules={[{ required: true, message: 'Username harus diisi' }]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="email"
          label="Email"
          rules={[
            { required: true, message: 'Email harus diisi' },
            { type: 'email', message: 'Email tidak valid' },
          ]}
        >
          <Input />
        </Form.Item>

        {!user && (
          <Form.Item
            name="password"
            label="Password"
            rules={[
              { required: true, message: 'Password harus diisi' },
              { min: 6, message: 'Password minimal 6 karakter' },
            ]}
          >
            <Input.Password />
          </Form.Item>
        )}

        <Form.Item
          name="role"
          label="Role"
          rules={[{ required: true, message: 'Role harus dipilih' }]}
        >
          <Select>
            <Select.Option value={UserRole.ADMIN}>Admin</Select.Option>
            <Select.Option value={UserRole.TECHNICIAN}>Technician</Select.Option>
            <Select.Option value={UserRole.VIEWER}>Viewer</Select.Option>
          </Select>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

- [ ] **Step 2: Create UserTable component**

Create `frontend/src/presentation/components/users/UserTable.tsx`:

```typescript
import { Table, Button, Space, Tag, Popconfirm } from 'antd';
import { EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { User } from '@/domain/entities';
import type { ColumnsType } from 'antd/es/table';

interface UserTableProps {
  users: User[];
  loading: boolean;
  onEdit: (user: User) => void;
  onDelete: (id: string) => void;
}

export function UserTable({ users, loading, onEdit, onDelete }: UserTableProps) {
  const columns: ColumnsType<User> = [
    {
      title: 'Username',
      dataIndex: 'username',
      key: 'username',
    },
    {
      title: 'Email',
      dataIndex: 'email',
      key: 'email',
    },
    {
      title: 'Role',
      dataIndex: 'role',
      key: 'role',
      render: (role: string) => {
        const color = role === 'admin' ? 'red' : role === 'technician' ? 'blue' : 'green';
        return <Tag color={color}>{role.toUpperCase()}</Tag>;
      },
    },
    {
      title: 'Created At',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (date: string) => new Date(date).toLocaleDateString('id-ID'),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => onEdit(record)}
          >
            Edit
          </Button>
          <Popconfirm
            title="Hapus user ini?"
            onConfirm={() => onDelete(record.id)}
            okText="Ya"
            cancelText="Tidak"
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={users}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
    />
  );
}
```

- [ ] **Step 3: Create barrel export**

Create `frontend/src/presentation/components/users/index.ts`:

```typescript
export * from './UserTable';
export * from './UserModal';
```

- [ ] **Step 4: Create Users page**

Create `frontend/src/presentation/pages/Users.tsx`:

```typescript
import { useState } from 'react';
import { Button, Typography, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useUsers, useCreateUser, useUpdateUser, useDeleteUser } from '@/application/hooks';
import { UserTable, UserModal } from '../components/users';
import type { User, CreateUserDto, UpdateUserDto } from '@/domain/entities';

const { Title } = Typography;

export default function UsersPage() {
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedUser, setSelectedUser] = useState<User | undefined>();

  const { data: users, isLoading } = useUsers();
  const createMutation = useCreateUser();
  const updateMutation = useUpdateUser();
  const deleteMutation = useDeleteUser();

  const handleCreate = () => {
    setSelectedUser(undefined);
    setModalOpen(true);
  };

  const handleEdit = (user: User) => {
    setSelectedUser(user);
    setModalOpen(true);
  };

  const handleSubmit = (data: CreateUserDto | UpdateUserDto) => {
    if (selectedUser) {
      updateMutation.mutate(
        { id: selectedUser.id, data: data as UpdateUserDto },
        {
          onSuccess: () => {
            message.success('User berhasil diupdate');
            setModalOpen(false);
          },
          onError: () => {
            message.error('Gagal update user');
          },
        }
      );
    } else {
      createMutation.mutate(data as CreateUserDto, {
        onSuccess: () => {
          message.success('User berhasil dibuat');
          setModalOpen(false);
        },
        onError: () => {
          message.error('Gagal membuat user');
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success('User berhasil dihapus');
      },
      onError: () => {
        message.error('Gagal menghapus user');
      },
    });
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>
          Users Management
        </Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          Create User
        </Button>
      </div>

      <UserTable
        users={users || []}
        loading={isLoading}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

      <UserModal
        open={modalOpen}
        user={selectedUser}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  );
}
```

- [ ] **Step 5: Update routes**

Edit `frontend/src/presentation/routes/index.tsx`, replace users placeholder:

```typescript
import UsersPage from '../pages/Users';

// In children array, replace:
{
  path: 'users',
  element: <UsersPage />,
},
```

- [ ] **Step 6: Test users page**

```bash
npm run dev
```

Expected:
- Users table shows all users
- Create button opens modal
- Edit button opens modal with user data
- Delete shows confirmation
- Success/error messages work

- [ ] **Step 7: Commit**

```bash
git add frontend/src/presentation/
git commit -m "feat(frontend): implement users management page

- Add users table with actions
- Add create/edit user modal with validation
- Add delete confirmation
- Add role selection (Admin/Technician/Viewer)
- Add success/error notifications"
```

---

### Task 12: Presentation Layer - Sites Management (CRUD)

**Files:**
- Create: `frontend/src/presentation/pages/Sites.tsx`
- Create: `frontend/src/presentation/components/sites/SiteTable.tsx`
- Create: `frontend/src/presentation/components/sites/SiteModal.tsx`
- Create: `frontend/src/presentation/components/sites/index.ts`
- Modify: `frontend/src/presentation/routes/index.tsx`

**Interfaces:**
- Consumes:
  - `useSites()`, `useCreateSite()`, `useUpdateSite()`, `useDeleteSite()` from application
  - `Site`, `CreateSiteDto`, `UpdateSiteDto` from domain
- Produces:
  - Sites list table with OLT count
  - Create/Edit site modal
  - Delete confirmation

- [ ] **Step 1: Create SiteModal component**

Create `frontend/src/presentation/components/sites/SiteModal.tsx`:

```typescript
import { Modal, Form, Input } from 'antd';
import { type Site, type CreateSiteDto, type UpdateSiteDto } from '@/domain/entities';
import { useEffect } from 'react';

interface SiteModalProps {
  open: boolean;
  site?: Site;
  onClose: () => void;
  onSubmit: (data: CreateSiteDto | UpdateSiteDto) => void;
  loading: boolean;
}

export function SiteModal({ open, site, onClose, onSubmit, loading }: SiteModalProps) {
  const [form] = Form.useForm();

  useEffect(() => {
    if (site) {
      form.setFieldsValue({
        name: site.name,
        location: site.location,
        description: site.description,
      });
    } else {
      form.resetFields();
    }
  }, [site, form]);

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      onSubmit(values);
    });
  };

  return (
    <Modal
      title={site ? 'Edit Site' : 'Create Site'}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="name"
          label="Site Name"
          rules={[{ required: true, message: 'Nama site harus diisi' }]}
        >
          <Input />
        </Form.Item>

        <Form.Item name="location" label="Location">
          <Input />
        </Form.Item>

        <Form.Item name="description" label="Description">
          <Input.TextArea rows={4} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

- [ ] **Step 2: Create SiteTable component**

Create `frontend/src/presentation/components/sites/SiteTable.tsx`:

```typescript
import { Table, Button, Space, Badge, Popconfirm } from 'antd';
import { EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { Site } from '@/domain/entities';
import type { ColumnsType } from 'antd/es/table';

interface SiteTableProps {
  sites: Site[];
  loading: boolean;
  onEdit: (site: Site) => void;
  onDelete: (id: string) => void;
}

export function SiteTable({ sites, loading, onEdit, onDelete }: SiteTableProps) {
  const columns: ColumnsType<Site> = [
    {
      title: 'Site Name',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'Location',
      dataIndex: 'location',
      key: 'location',
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'OLT Count',
      dataIndex: 'oltCount',
      key: 'oltCount',
      render: (count: number) => <Badge count={count} showZero color="blue" />,
    },
    {
      title: 'Created At',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (date: string) => new Date(date).toLocaleDateString('id-ID'),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => onEdit(record)}
          >
            Edit
          </Button>
          <Popconfirm
            title="Hapus site ini?"
            description="OLT yang terhubung dengan site ini tidak akan terhapus"
            onConfirm={() => onDelete(record.id)}
            okText="Ya"
            cancelText="Tidak"
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={sites}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
    />
  );
}
```

- [ ] **Step 3: Create barrel export**

Create `frontend/src/presentation/components/sites/index.ts`:

```typescript
export * from './SiteTable';
export * from './SiteModal';
```

- [ ] **Step 4: Create Sites page**

Create `frontend/src/presentation/pages/Sites.tsx`:

```typescript
import { useState } from 'react';
import { Button, Typography, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useSites, useCreateSite, useUpdateSite, useDeleteSite } from '@/application/hooks';
import { SiteTable, SiteModal } from '../components/sites';
import type { Site, CreateSiteDto, UpdateSiteDto } from '@/domain/entities';

const { Title } = Typography;

export default function SitesPage() {
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedSite, setSelectedSite] = useState<Site | undefined>();

  const { data: sites, isLoading } = useSites();
  const createMutation = useCreateSite();
  const updateMutation = useUpdateSite();
  const deleteMutation = useDeleteSite();

  const handleCreate = () => {
    setSelectedSite(undefined);
    setModalOpen(true);
  };

  const handleEdit = (site: Site) => {
    setSelectedSite(site);
    setModalOpen(true);
  };

  const handleSubmit = (data: CreateSiteDto | UpdateSiteDto) => {
    if (selectedSite) {
      updateMutation.mutate(
        { id: selectedSite.id, data: data as UpdateSiteDto },
        {
          onSuccess: () => {
            message.success('Site berhasil diupdate');
            setModalOpen(false);
          },
          onError: () => {
            message.error('Gagal update site');
          },
        }
      );
    } else {
      createMutation.mutate(data as CreateSiteDto, {
        onSuccess: () => {
          message.success('Site berhasil dibuat');
          setModalOpen(false);
        },
        onError: () => {
          message.error('Gagal membuat site');
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success('Site berhasil dihapus');
      },
      onError: () => {
        message.error('Gagal menghapus site');
      },
    });
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>
          Sites Management
        </Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          Create Site
        </Button>
      </div>

      <SiteTable
        sites={sites || []}
        loading={isLoading}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

      <SiteModal
        open={modalOpen}
        site={selectedSite}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  );
}
```

- [ ] **Step 5: Update routes**

Edit `frontend/src/presentation/routes/index.tsx`, replace sites placeholder:

```typescript
import SitesPage from '../pages/Sites';

// In children array, replace:
{
  path: 'sites',
  element: <SitesPage />,
},
```

- [ ] **Step 6: Test sites page**

```bash
npm run dev
```

Expected:
- Sites table shows all sites with OLT count
- Create button opens modal
- Edit button opens modal with site data
- Delete shows confirmation

- [ ] **Step 7: Commit**

```bash
git add frontend/src/presentation/
git commit -m "feat(frontend): implement sites management page

- Add sites table with OLT count badge
- Add create/edit site modal with validation
- Add delete confirmation
- Add location and description fields
- Add success/error notifications"
```

---

### Task 13: Presentation Layer - OLTs Management (CRUD)

**Files:**
- Create: `frontend/src/presentation/pages/Olts.tsx`
- Create: `frontend/src/presentation/components/olts/OltTable.tsx`
- Create: `frontend/src/presentation/components/olts/OltModal.tsx`
- Create: `frontend/src/presentation/components/olts/index.ts`
- Modify: `frontend/src/presentation/routes/index.tsx`

**Interfaces:**
- Consumes:
  - `useOlts()`, `useCreateOlt()`, `useUpdateOlt()`, `useDeleteOlt()`, `useSites()` from application
  - `Olt`, `CreateOltDto`, `UpdateOltDto`, `OltProtocol`, `OltStatus` from domain
- Produces:
  - OLTs list table with status badges
  - Create/Edit OLT modal with protocol/site selection
  - Delete confirmation

- [ ] **Step 1: Create OltModal component**

Create `frontend/src/presentation/components/olts/OltModal.tsx`:

```typescript
import { Modal, Form, Input, Select, InputNumber } from 'antd';
import { type Olt, type CreateOltDto, type UpdateOltDto, OltProtocol } from '@/domain/entities';
import { useSites } from '@/application/hooks';
import { useEffect } from 'react';

interface OltModalProps {
  open: boolean;
  olt?: Olt;
  onClose: () => void;
  onSubmit: (data: CreateOltDto | UpdateOltDto) => void;
  loading: boolean;
}

export function OltModal({ open, olt, onClose, onSubmit, loading }: OltModalProps) {
  const [form] = Form.useForm();
  const { data: sites } = useSites();

  useEffect(() => {
    if (olt) {
      form.setFieldsValue({
        siteId: olt.siteId,
        name: olt.name,
        ipAddress: olt.ipAddress,
        protocol: olt.protocol,
        username: olt.username,
        snmpCommunity: olt.snmpCommunity,
        sshPort: olt.sshPort,
        telnetPort: olt.telnetPort,
        snmpPort: olt.snmpPort,
      });
    } else {
      form.resetFields();
      form.setFieldsValue({
        protocol: OltProtocol.SSH,
        sshPort: 22,
        telnetPort: 23,
        snmpPort: 161,
      });
    }
  }, [olt, form]);

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      onSubmit(values);
    });
  };

  return (
    <Modal
      title={olt ? 'Edit OLT' : 'Create OLT'}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
      width={600}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="siteId"
          label="Site"
          rules={[{ required: true, message: 'Site harus dipilih' }]}
        >
          <Select placeholder="Pilih site">
            {sites?.map((site) => (
              <Select.Option key={site.id} value={site.id}>
                {site.name}
              </Select.Option>
            ))}
          </Select>
        </Form.Item>

        <Form.Item
          name="name"
          label="OLT Name"
          rules={[{ required: true, message: 'Nama OLT harus diisi' }]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="ipAddress"
          label="IP Address"
          rules={[
            { required: true, message: 'IP Address harus diisi' },
            { pattern: /^(\d{1,3}\.){3}\d{1,3}$/, message: 'IP Address tidak valid' },
          ]}
        >
          <Input placeholder="192.168.1.1" />
        </Form.Item>

        <Form.Item
          name="protocol"
          label="Protocol"
          rules={[{ required: true, message: 'Protocol harus dipilih' }]}
        >
          <Select>
            <Select.Option value={OltProtocol.SSH}>SSH</Select.Option>
            <Select.Option value={OltProtocol.TELNET}>Telnet</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item
          name="username"
          label="Username"
          rules={[{ required: true, message: 'Username harus diisi' }]}
        >
          <Input />
        </Form.Item>

        {!olt && (
          <Form.Item
            name="password"
            label="Password"
            rules={[{ required: true, message: 'Password harus diisi' }]}
          >
            <Input.Password />
          </Form.Item>
        )}

        <Form.Item name="snmpCommunity" label="SNMP Community">
          <Input placeholder="public" />
        </Form.Item>

        <Form.Item name="sshPort" label="SSH Port">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="telnetPort" label="Telnet Port">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="snmpPort" label="SNMP Port">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

- [ ] **Step 2: Create OltTable component**

Create `frontend/src/presentation/components/olts/OltTable.tsx`:

```typescript
import { Table, Button, Space, Tag, Popconfirm } from 'antd';
import { EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { type Olt, OltStatus } from '@/domain/entities';
import type { ColumnsType } from 'antd/es/table';

interface OltTableProps {
  olts: Olt[];
  loading: boolean;
  onEdit: (olt: Olt) => void;
  onDelete: (id: string) => void;
}

export function OltTable({ olts, loading, onEdit, onDelete }: OltTableProps) {
  const getStatusColor = (status: OltStatus) => {
    switch (status) {
      case OltStatus.ONLINE:
        return 'green';
      case OltStatus.OFFLINE:
        return 'red';
      case OltStatus.ERROR:
        return 'orange';
      default:
        return 'default';
    }
  };

  const columns: ColumnsType<Olt> = [
    {
      title: 'OLT Name',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'Site',
      dataIndex: 'siteName',
      key: 'siteName',
    },
    {
      title: 'IP Address',
      dataIndex: 'ipAddress',
      key: 'ipAddress',
    },
    {
      title: 'Protocol',
      dataIndex: 'protocol',
      key: 'protocol',
      render: (protocol: string) => protocol.toUpperCase(),
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status: OltStatus) => (
        <Tag color={getStatusColor(status)}>{status.toUpperCase()}</Tag>
      ),
    },
    {
      title: 'Last Seen',
      dataIndex: 'lastSeen',
      key: 'lastSeen',
      render: (date: string | null) =>
        date ? new Date(date).toLocaleString('id-ID') : '-',
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => onEdit(record)}
          >
            Edit
          </Button>
          <Popconfirm
            title="Hapus OLT ini?"
            onConfirm={() => onDelete(record.id)}
            okText="Ya"
            cancelText="Tidak"
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Table
      columns={columns}
      dataSource={olts}
      loading={loading}
      rowKey="id"
      pagination={{ pageSize: 10 }}
    />
  );
}
```

- [ ] **Step 3: Create barrel export**

Create `frontend/src/presentation/components/olts/index.ts`:

```typescript
export * from './OltTable';
export * from './OltModal';
```

- [ ] **Step 4: Create Olts page**

Create `frontend/src/presentation/pages/Olts.tsx`:

```typescript
import { useState } from 'react';
import { Button, Typography, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useOlts, useCreateOlt, useUpdateOlt, useDeleteOlt } from '@/application/hooks';
import { OltTable, OltModal } from '../components/olts';
import type { Olt, CreateOltDto, UpdateOltDto } from '@/domain/entities';

const { Title } = Typography;

export default function OltsPage() {
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedOlt, setSelectedOlt] = useState<Olt | undefined>();

  const { data: olts, isLoading } = useOlts();
  const createMutation = useCreateOlt();
  const updateMutation = useUpdateOlt();
  const deleteMutation = useDeleteOlt();

  const handleCreate = () => {
    setSelectedOlt(undefined);
    setModalOpen(true);
  };

  const handleEdit = (olt: Olt) => {
    setSelectedOlt(olt);
    setModalOpen(true);
  };

  const handleSubmit = (data: CreateOltDto | UpdateOltDto) => {
    if (selectedOlt) {
      updateMutation.mutate(
        { id: selectedOlt.id, data: data as UpdateOltDto },
        {
          onSuccess: () => {
            message.success('OLT berhasil diupdate');
            setModalOpen(false);
          },
          onError: () => {
            message.error('Gagal update OLT');
          },
        }
      );
    } else {
      createMutation.mutate(data as CreateOltDto, {
        onSuccess: () => {
          message.success('OLT berhasil dibuat');
          setModalOpen(false);
        },
        onError: () => {
          message.error('Gagal membuat OLT');
        },
      });
    }
  };

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        message.success('OLT berhasil dihapus');
      },
      onError: () => {
        message.error('Gagal menghapus OLT');
      },
    });
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>
          OLTs Management
        </Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          Create OLT
        </Button>
      </div>

      <OltTable
        olts={olts || []}
        loading={isLoading}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

      <OltModal
        open={modalOpen}
        olt={selectedOlt}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  );
}
```

- [ ] **Step 5: Update routes**

Edit `frontend/src/presentation/routes/index.tsx`, replace olts placeholder:

```typescript
import OltsPage from '../pages/Olts';

// In children array, replace:
{
  path: 'olts',
  element: <OltsPage />,
},
```

- [ ] **Step 6: Test OLTs page**

```bash
npm run dev
```

Expected:
- OLTs table shows all OLTs with status
- Create button opens modal with site selection
- Protocol selection (SSH/Telnet)
- Port configuration fields

- [ ] **Step 7: Commit**

```bash
git add frontend/src/presentation/
git commit -m "feat(frontend): implement OLTs management page

- Add OLTs table with status badges
- Add create/edit OLT modal with protocol selection
- Add site selection dropdown
- Add IP validation and port configuration
- Add status indicators (Online/Offline/Error)
- Add success/error notifications"
```

---

### Task 14: Test Setup & Build Verification

**Files:**
- Create: `frontend/src/test/setup.ts`
- Modify: `frontend/vite.config.ts`

**Interfaces:**
- Consumes: Vitest configuration
- Produces: Working test environment, successful production build

- [ ] **Step 1: Create test setup file**

Create `frontend/src/test/setup.ts`:

```typescript
import { expect, afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';

afterEach(() => {
  cleanup();
});

global.matchMedia = vi.fn().mockImplementation((query) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}));
```

- [ ] **Step 2: Run all tests**

```bash
npm test
```

Expected: All tests PASS

- [ ] **Step 3: Build for production**

```bash
npm run build
```

Expected: Build completes successfully, output in `dist/` folder

- [ ] **Step 4: Check bundle size**

```bash
ls -lh frontend/dist/assets/
```

Expected: Main JS bundle < 200KB gzipped

- [ ] **Step 5: Test production build**

```bash
npm run preview
```

Expected: Production build runs at http://localhost:4173

- [ ] **Step 6: Commit**

```bash
git add frontend/
git commit -m "feat(frontend): add test setup and verify build

- Add Vitest test setup with jsdom
- Configure matchMedia mock for Ant Design
- Verify all tests pass
- Verify production build succeeds
- Check bundle size meets target"
```

---

### Task 15: Docker Setup & Deployment Configuration

**Files:**
- Create: `frontend/Dockerfile`
- Create: `frontend/nginx.conf`
- Modify: `docker-compose.yml` (root level)

**Interfaces:**
- Consumes: Built frontend assets from `dist/`
- Produces:
  - Nginx-based Docker container
  - docker-compose service for frontend
  - Production-ready deployment

- [ ] **Step 1: Create Nginx configuration**

Create `frontend/nginx.conf`:

```nginx
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api {
        proxy_pass http://api:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;
    gzip_min_length 1000;
}
```

- [ ] **Step 2: Create Dockerfile**

Create `frontend/Dockerfile`:

```dockerfile
FROM node:20-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

- [ ] **Step 3: Update docker-compose.yml**

Edit `docker-compose.yml` in root, add frontend service:

```yaml
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    container_name: tikman-frontend
    ports:
      - "3000:80"
    depends_on:
      api:
        condition: service_healthy
    networks:
      - tikman-network
    restart: unless-stopped
```

- [ ] **Step 4: Test Docker build**

```bash
cd frontend
docker build -t tikman-frontend .
```

Expected: Image builds successfully

- [ ] **Step 5: Test full stack with docker-compose**

```bash
cd ..
docker-compose up --build
```

Expected:
- All services start successfully
- Frontend accessible at http://localhost:3000
- Backend API at http://localhost:8080
- Frontend can communicate with backend

- [ ] **Step 6: Create README for frontend**

Create `frontend/README.md`:

```markdown
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
```

- [ ] **Step 7: Commit**

```bash
git add frontend/ docker-compose.yml
git commit -m "feat(frontend): add Docker setup and deployment config

- Add multi-stage Dockerfile with Node builder
- Add Nginx configuration with API proxy
- Add frontend service to docker-compose
- Add gzip compression for assets
- Add README with development instructions"
```

---

## Plan Complete

All 15 tasks defined. Frontend implementation covers:

1. ✅ Project scaffolding & configuration
2. ✅ Domain layer (entities & repository interfaces)
3. ✅ Infrastructure layer (API client & error handling)
4. ✅ Infrastructure layer (repository implementations)
5. ✅ Application layer (auth store with Zustand)
6. ✅ Application layer (React Query hooks)
7. ✅ Presentation layer (router & protected routes)
8. ✅ Presentation layer (layout components)
9. ✅ Presentation layer (login page)
10. ✅ Presentation layer (dashboard page)
11. ✅ Presentation layer (users management CRUD)
12. ✅ Presentation layer (sites management CRUD)
13. ✅ Presentation layer (OLTs management CRUD)
14. ✅ Test setup & build verification
15. ✅ Docker & deployment configuration

**Execution Options:**

1. **Subagent-Driven Development (Recommended)**: Fresh subagent per task with task reviews
2. **Inline Execution**: Execute in this session with checkpoints

Which approach would you like to use?
