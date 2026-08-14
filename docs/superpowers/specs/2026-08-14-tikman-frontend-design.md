# TikMan OLT Provisioning - Frontend Application Design Specification

**Date**: 2026-08-14  
**Project**: TikMan - ZTE OLT Provisioning System Frontend  
**Version**: 1.0

## 1. Overview

Frontend web application untuk ZTE OLT Provisioning System dengan React + TypeScript, menggunakan Clean Architecture principles untuk maintainability dan scalability jangka panjang.

### Key Features
- Session-based authentication dengan cookie
- Role-based access control (Admin, Technician, Viewer)
- User management (CRUD)
- Site management (CRUD)
- OLT management (CRUD)
- Real-time dashboard monitoring
- Responsive design untuk desktop & tablet
- Production-ready dengan proper error handling & loading states

### Technology Stack
- **Framework**: React 18 + TypeScript 5
- **Build Tool**: Vite 5 (fast HMR, optimized builds)
- **UI Library**: Ant Design 5 (comprehensive component library)
- **Routing**: React Router v6 (protected routes, RBAC)
- **State Management**: 
  - React Query (server state, caching, mutations)
  - Zustand (client state, auth)
- **HTTP Client**: Axios (interceptors, request/response mapping)
- **Testing**: Vitest + React Testing Library
- **Code Quality**: ESLint + Prettier

## 2. Clean Architecture Principles

### Architecture Layers

```
┌─────────────────────────────────────┐
│   Presentation Layer                │  ← React Components, Pages, UI
│   - Components, Pages, Routes       │
└──────────────┬──────────────────────┘
               │ uses
┌──────────────▼──────────────────────┐
│   Application Layer                 │  ← Business Logic
│   - Hooks (use cases)               │
│   - Services                        │
│   - Contexts                        │
└──────────────┬──────────────────────┘
               │ uses
┌──────────────▼──────────────────────┐
│   Domain Layer                      │  ← Core Models (Pure TypeScript)
│   - Entities (User, Site, OLT)     │
│   - Repository Interfaces           │
└──────────────┬──────────────────────┘
               │ implemented by
┌──────────────▼──────────────────────┐
│   Infrastructure Layer              │  ← External Dependencies
│   - Repository Implementations      │
│   - API Client (Axios)              │
│   - HTTP Interceptors               │
└─────────────────────────────────────┘
```

**Dependency Rule**: Inner layers don't know outer layers exist
- Domain: Pure TypeScript types/interfaces (no React, no HTTP)
- Application: Business logic using domain entities
- Infrastructure: API calls, implements domain interfaces
- Presentation: React components, uses application hooks

## 3. Project Structure

```
frontend/
├── src/
│   ├── domain/                      # Domain Layer (Pure TS)
│   │   ├── entities/
│   │   │   ├── User.ts             # User entity + UserRole enum
│   │   │   ├── Site.ts             # Site entity
│   │   │   ├── Olt.ts              # OLT entity + enums
│   │   │   └── index.ts
│   │   └── repositories/            # Repository interfaces
│   │       ├── IAuthRepository.ts
│   │       ├── IUserRepository.ts
│   │       ├── ISiteRepository.ts
│   │       ├── IOltRepository.ts
│   │       └── index.ts
│   │
│   ├── infrastructure/              # Infrastructure Layer
│   │   ├── http/
│   │   │   ├── apiClient.ts        # Axios instance + config
│   │   │   ├── endpoints.ts        # API endpoint constants
│   │   │   ├── interceptors.ts     # Request/response interceptors
│   │   │   └── errorMapper.ts      # Map API errors to domain errors
│   │   └── repositories/            # Repository implementations
│   │       ├── AuthRepository.ts    # Implements IAuthRepository
│   │       ├── UserRepository.ts
│   │       ├── SiteRepository.ts
│   │       ├── OltRepository.ts
│   │       └── index.ts
│   │
│   ├── application/                 # Application Layer
│   │   ├── contexts/
│   │   │   ├── AuthContext.tsx     # Auth state + user info
│   │   │   ├── RepositoryContext.tsx  # DI container for repos
│   │   │   └── index.ts
│   │   ├── hooks/                   # Custom hooks (use cases)
│   │   │   ├── auth/
│   │   │   │   ├── useAuth.ts      # Login, logout, check auth
│   │   │   │   └── useCurrentUser.ts
│   │   │   ├── users/
│   │   │   │   ├── useUsers.ts     # List users (React Query)
│   │   │   │   ├── useCreateUser.ts
│   │   │   │   ├── useUpdateUser.ts
│   │   │   │   ├── useDeleteUser.ts
│   │   │   │   └── index.ts
│   │   │   ├── sites/
│   │   │   │   ├── useSites.ts
│   │   │   │   ├── useCreateSite.ts
│   │   │   │   ├── useUpdateSite.ts
│   │   │   │   ├── useDeleteSite.ts
│   │   │   │   └── index.ts
│   │   │   ├── olts/
│   │   │   │   ├── useOlts.ts
│   │   │   │   ├── useCreateOlt.ts
│   │   │   │   ├── useUpdateOlt.ts
│   │   │   │   ├── useDeleteOlt.ts
│   │   │   │   └── index.ts
│   │   │   └── index.ts
│   │   └── stores/
│   │       └── authStore.ts        # Zustand store for auth state
│   │
│   ├── presentation/                # Presentation Layer
│   │   ├── components/
│   │   │   ├── layouts/
│   │   │   │   ├── AppLayout/
│   │   │   │   │   ├── index.tsx
│   │   │   │   │   ├── Sidebar.tsx
│   │   │   │   │   ├── Header.tsx
│   │   │   │   │   ├── Breadcrumb.tsx
│   │   │   │   │   └── styles.module.css
│   │   │   │   └── AuthLayout/
│   │   │   │       └── index.tsx
│   │   │   ├── features/            # Feature-specific components
│   │   │   │   ├── auth/
│   │   │   │   │   ├── LoginForm.tsx
│   │   │   │   │   └── LogoutButton.tsx
│   │   │   │   ├── dashboard/
│   │   │   │   │   ├── StatsCard.tsx
│   │   │   │   │   ├── RecentActivity.tsx
│   │   │   │   │   └── OltStatusChart.tsx
│   │   │   │   ├── users/
│   │   │   │   │   ├── UserTable.tsx
│   │   │   │   │   ├── UserForm.tsx
│   │   │   │   │   ├── UserModal.tsx
│   │   │   │   │   ├── UserFilters.tsx
│   │   │   │   │   └── index.ts
│   │   │   │   ├── sites/
│   │   │   │   │   ├── SiteTable.tsx
│   │   │   │   │   ├── SiteForm.tsx
│   │   │   │   │   ├── SiteModal.tsx
│   │   │   │   │   ├── SiteCard.tsx
│   │   │   │   │   └── index.ts
│   │   │   │   └── olts/
│   │   │   │       ├── OltTable.tsx
│   │   │   │       ├── OltForm.tsx
│   │   │   │       ├── OltModal.tsx
│   │   │   │       ├── OltCard.tsx
│   │   │   │       ├── OltStatusBadge.tsx
│   │   │   │       └── index.ts
│   │   │   └── common/              # Reusable UI components
│   │   │       ├── DataTable/
│   │   │       │   ├── index.tsx
│   │   │       │   └── types.ts
│   │   │       ├── PageHeader/
│   │   │       │   └── index.tsx
│   │   │       ├── EmptyState/
│   │   │       │   └── index.tsx
│   │   │       ├── ErrorBoundary/
│   │   │       │   └── index.tsx
│   │   │       ├── LoadingSpinner/
│   │   │       │   └── index.tsx
│   │   │       ├── ConfirmDialog/
│   │   │       │   └── index.tsx
│   │   │       └── index.ts
│   │   ├── pages/                   # Route pages
│   │   │   ├── LoginPage.tsx
│   │   │   ├── DashboardPage.tsx
│   │   │   ├── UsersPage.tsx
│   │   │   ├── SitesPage.tsx
│   │   │   ├── OltsPage.tsx
│   │   │   └── NotFoundPage.tsx
│   │   └── routes/
│   │       ├── AppRoutes.tsx        # All routes definition
│   │       ├── ProtectedRoute.tsx   # Auth guard
│   │       ├── RoleRoute.tsx        # RBAC guard
│   │       └── index.ts
│   │
│   ├── shared/                      # Shared utilities
│   │   ├── constants/
│   │   │   ├── routes.ts           # Route paths
│   │   │   ├── permissions.ts      # Role permissions matrix
│   │   │   └── index.ts
│   │   ├── utils/
│   │   │   ├── dateFormatter.ts
│   │   │   ├── validators.ts
│   │   │   └── index.ts
│   │   ├── config/
│   │   │   ├── env.ts              # Environment variables
│   │   │   ├── theme.ts            # Ant Design theme
│   │   │   └── queryClient.ts      # React Query config
│   │   └── types/
│   │       └── common.ts           # Shared types
│   │
│   ├── App.tsx                      # Root component
│   ├── main.tsx                     # Entry point
│   └── vite-env.d.ts
│
├── public/
│   └── favicon.ico
├── .env.example
├── .env.development
├── .env.production
├── package.json
├── tsconfig.json
├── vite.config.ts
├── eslint.config.js
├── prettier.config.js
├── Dockerfile
└── nginx.conf
```

## 4. Domain Layer

### Entities

```typescript
// domain/entities/User.ts
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

```typescript
// domain/entities/Site.ts
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

```typescript
// domain/entities/Olt.ts
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
  // password encrypted, not exposed
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

### Repository Interfaces

```typescript
// domain/repositories/IAuthRepository.ts
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

```typescript
// domain/repositories/IUserRepository.ts
export interface IUserRepository {
  getAll(): Promise<User[]>;
  getById(id: string): Promise<User>;
  create(data: CreateUserDto): Promise<User>;
  update(id: string, data: UpdateUserDto): Promise<User>;
  delete(id: string): Promise<void>;
}
```

```typescript
// domain/repositories/ISiteRepository.ts
export interface ISiteRepository {
  getAll(): Promise<Site[]>;
  getById(id: string): Promise<Site>;
  create(data: CreateSiteDto): Promise<Site>;
  update(id: string, data: UpdateSiteDto): Promise<Site>;
  delete(id: string): Promise<void>;
}
```

```typescript
// domain/repositories/IOltRepository.ts
export interface IOltRepository {
  getAll(): Promise<Olt[]>;
  getBySite(siteId: string): Promise<Olt[]>;
  getById(id: string): Promise<Olt>;
  create(data: CreateOltDto): Promise<Olt>;
  update(id: string, data: UpdateOltDto): Promise<Olt>;
  delete(id: string): Promise<void>;
}
```

## 5. Infrastructure Layer

### API Client

```typescript
// infrastructure/http/apiClient.ts
import axios from 'axios';
import { env } from '@/shared/config/env';

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
    
    // Auto logout on 401
    if (error.response?.status === 401) {
      const { authStore } = await import('@/application/stores/authStore');
      authStore.getState().logout();
    }
    
    return Promise.reject(mappedError);
  }
);
```

```typescript
// infrastructure/http/endpoints.ts
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

```typescript
// infrastructure/http/errorMapper.ts
import { AxiosError } from 'axios';

export class ApiError extends Error {
  constructor(
    public statusCode: number,
    public code: string,
    public details?: Record<string, any>
  ) {
    super();
    this.name = 'ApiError';
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
  }
}

export class NotFoundError extends ApiError {
  constructor(resource: string) {
    super(404, 'NOT_FOUND', { resource });
    this.name = 'NotFoundError';
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

### Repository Implementations

```typescript
// infrastructure/repositories/AuthRepository.ts
import { apiClient } from '../http/apiClient';
import { API_ENDPOINTS } from '../http/endpoints';
import type { 
  IAuthRepository, 
  LoginCredentials, 
  LoginResponse 
} from '@/domain/repositories';
import type { User } from '@/domain/entities';

export class AuthRepository implements IAuthRepository {
  async login(credentials: LoginCredentials): Promise<LoginResponse> {
    const response = await apiClient.post(
      API_ENDPOINTS.AUTH_LOGIN,
      credentials
    );
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

```typescript
// infrastructure/repositories/UserRepository.ts
import { apiClient } from '../http/apiClient';
import { API_ENDPOINTS } from '../http/endpoints';
import type { IUserRepository } from '@/domain/repositories';
import type { User, CreateUserDto, UpdateUserDto } from '@/domain/entities';

export class UserRepository implements IUserRepository {
  async getAll(): Promise<User[]> {
    const response = await apiClient.get(API_ENDPOINTS.USERS);
    return response.data;
  }

  async getById(id: string): Promise<User> {
    const response = await apiClient.get(API_ENDPOINTS.USER_BY_ID(id));
    return response.data;
  }

  async create(data: CreateUserDto): Promise<User> {
    const response = await apiClient.post(API_ENDPOINTS.USERS, data);
    return response.data;
  }

  async update(id: string, data: UpdateUserDto): Promise<User> {
    const response = await apiClient.put(API_ENDPOINTS.USER_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.USER_BY_ID(id));
  }
}
```

## 6. Application Layer

### Auth Store (Zustand)

```typescript
// application/stores/authStore.ts
import { create } from 'zustand';
import type { User } from '@/domain/entities';

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  setUser: (user: User | null) => void;
  setLoading: (loading: boolean) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  
  setUser: (user) => set({ 
    user, 
    isAuthenticated: !!user,
    isLoading: false,
  }),
  
  setLoading: (loading) => set({ isLoading: loading }),
  
  logout: () => set({ 
    user: null, 
    isAuthenticated: false,
    isLoading: false,
  }),
}));
```

### Repository Context (DI)

```typescript
// application/contexts/RepositoryContext.tsx
import React, { createContext, useContext } from 'react';
import { AuthRepository } from '@/infrastructure/repositories/AuthRepository';
import { UserRepository } from '@/infrastructure/repositories/UserRepository';
import { SiteRepository } from '@/infrastructure/repositories/SiteRepository';
import { OltRepository } from '@/infrastructure/repositories/OltRepository';
import type { 
  IAuthRepository, 
  IUserRepository, 
  ISiteRepository,
  IOltRepository,
} from '@/domain/repositories';

interface Repositories {
  auth: IAuthRepository;
  users: IUserRepository;
  sites: ISiteRepository;
  olts: IOltRepository;
}

const repositories: Repositories = {
  auth: new AuthRepository(),
  users: new UserRepository(),
  sites: new SiteRepository(),
  olts: new OltRepository(),
};

const RepositoryContext = createContext<Repositories>(repositories);

export const RepositoryProvider: React.FC<{ children: React.ReactNode }> = ({ 
  children 
}) => {
  return (
    <RepositoryContext.Provider value={repositories}>
      {children}
    </RepositoryContext.Provider>
  );
};

export const useRepositories = () => useContext(RepositoryContext);
export const useAuthRepository = () => useRepositories().auth;
export const useUserRepository = () => useRepositories().users;
export const useSiteRepository = () => useRepositories().sites;
export const useOltRepository = () => useRepositories().olts;
```

### Custom Hooks

```typescript
// application/hooks/auth/useAuth.ts
import { useAuthStore } from '@/application/stores/authStore';
import { useAuthRepository } from '@/application/contexts/RepositoryContext';
import { message } from 'antd';
import { useNavigate } from 'react-router-dom';

export function useAuth() {
  const { user, isAuthenticated, isLoading, setUser, setLoading, logout: clearAuth } = useAuthStore();
  const authRepository = useAuthRepository();
  const navigate = useNavigate();

  const login = async (username: string, password: string) => {
    try {
      setLoading(true);
      const response = await authRepository.login({ username, password });
      setUser(response.user);
      message.success('Login successful');
      navigate('/dashboard');
    } catch (error: any) {
      message.error(error.message || 'Login failed');
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const logout = async () => {
    try {
      await authRepository.logout();
      clearAuth();
      navigate('/login');
      message.success('Logged out successfully');
    } catch (error: any) {
      // Clear auth even if API call fails
      clearAuth();
      navigate('/login');
    }
  };

  const checkAuth = async () => {
    try {
      setLoading(true);
      const user = await authRepository.getCurrentUser();
      setUser(user);
    } catch (error) {
      clearAuth();
    } finally {
      setLoading(false);
    }
  };

  return {
    user,
    isAuthenticated,
    isLoading,
    login,
    logout,
    checkAuth,
  };
}
```

```typescript
// application/hooks/users/useUsers.ts
import { useQuery } from '@tanstack/react-query';
import { useUserRepository } from '@/application/contexts/RepositoryContext';

export function useUsers() {
  const userRepository = useUserRepository();

  return useQuery({
    queryKey: ['users'],
    queryFn: () => userRepository.getAll(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
```

```typescript
// application/hooks/users/useCreateUser.ts
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useUserRepository } from '@/application/contexts/RepositoryContext';
import { message } from 'antd';
import type { CreateUserDto } from '@/domain/entities';

export function useCreateUser() {
  const userRepository = useUserRepository();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateUserDto) => userRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      message.success('User created successfully');
    },
    onError: (error: any) => {
      message.error(error.message || 'Failed to create user');
    },
  });
}
```

## 7. Presentation Layer

### Routes

```typescript
// presentation/routes/AppRoutes.tsx
import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Spin } from 'antd';
import { ProtectedRoute } from './ProtectedRoute';
import { RoleRoute } from './RoleRoute';
import { UserRole } from '@/domain/entities';

// Lazy load pages
const LoginPage = lazy(() => import('../pages/LoginPage'));
const DashboardPage = lazy(() => import('../pages/DashboardPage'));
const UsersPage = lazy(() => import('../pages/UsersPage'));
const SitesPage = lazy(() => import('../pages/SitesPage'));
const OltsPage = lazy(() => import('../pages/OltsPage'));
const NotFoundPage = lazy(() => import('../pages/NotFoundPage'));

const PageLoader = () => (
  <div style={{ 
    display: 'flex', 
    justifyContent: 'center', 
    alignItems: 'center', 
    minHeight: '100vh' 
  }}>
    <Spin size="large" />
  </div>
);

export function AppRoutes() {
  return (
    <Suspense fallback={<PageLoader />}>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />
        
        {/* Protected routes */}
        <Route element={<ProtectedRoute />}>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          
          {/* Admin only */}
          <Route element={<RoleRoute allowedRoles={[UserRole.ADMIN]} />}>
            <Route path="/users" element={<UsersPage />} />
          </Route>
          
          {/* Admin & Technician */}
          <Route element={<RoleRoute allowedRoles={[UserRole.ADMIN, UserRole.TECHNICIAN]} />}>
            <Route path="/sites" element={<SitesPage />} />
            <Route path="/olts" element={<OltsPage />} />
          </Route>
        </Route>
        
        {/* 404 */}
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Suspense>
  );
}
```

```typescript
// presentation/routes/ProtectedRoute.tsx
import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '@/application/hooks/auth/useAuth';
import { Spin } from 'antd';
import { AppLayout } from '../components/layouts/AppLayout';

export function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div style={{ 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center', 
        minHeight: '100vh' 
      }}>
        <Spin size="large" tip="Loading..." />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return (
    <AppLayout>
      <Outlet />
    </AppLayout>
  );
}
```

```typescript
// presentation/routes/RoleRoute.tsx
import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '@/application/hooks/auth/useAuth';
import { Result, Button } from 'antd';
import { UserRole } from '@/domain/entities';

interface RoleRouteProps {
  allowedRoles: UserRole[];
}

export function RoleRoute({ allowedRoles }: RoleRouteProps) {
  const { user } = useAuth();

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  if (!allowedRoles.includes(user.role)) {
    return (
      <Result
        status="403"
        title="403"
        subTitle="Sorry, you are not authorized to access this page."
        extra={
          <Button type="primary" href="/dashboard">
            Back to Dashboard
          </Button>
        }
      />
    );
  }

  return <Outlet />;
}
```

### Layout Components

```typescript
// presentation/components/layouts/AppLayout/index.tsx
import { Layout } from 'antd';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { Breadcrumb } from './Breadcrumb';
import styles from './styles.module.css';

const { Content } = Layout;

interface AppLayoutProps {
  children: React.ReactNode;
}

export function AppLayout({ children }: AppLayoutProps) {
  return (
    <Layout className={styles.layout}>
      <Sidebar />
      <Layout>
        <Header />
        <Breadcrumb />
        <Content className={styles.content}>
          {children}
        </Content>
      </Layout>
    </Layout>
  );
}
```

```typescript
// presentation/components/layouts/AppLayout/Sidebar.tsx
import { Layout, Menu } from 'antd';
import {
  DashboardOutlined,
  UserOutlined,
  EnvironmentOutlined,
  ApiOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/application/hooks/auth/useAuth';
import { UserRole } from '@/domain/entities';

const { Sider } = Layout;

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user } = useAuth();

  const menuItems = [
    {
      key: '/dashboard',
      icon: <DashboardOutlined />,
      label: 'Dashboard',
      onClick: () => navigate('/dashboard'),
    },
    {
      key: '/users',
      icon: <UserOutlined />,
      label: 'Users',
      onClick: () => navigate('/users'),
      visible: user?.role === UserRole.ADMIN,
    },
    {
      key: '/sites',
      icon: <EnvironmentOutlined />,
      label: 'Sites',
      onClick: () => navigate('/sites'),
      visible: [UserRole.ADMIN, UserRole.TECHNICIAN].includes(user?.role!),
    },
    {
      key: '/olts',
      icon: <ApiOutlined />,
      label: 'OLTs',
      onClick: () => navigate('/olts'),
      visible: [UserRole.ADMIN, UserRole.TECHNICIAN].includes(user?.role!),
    },
  ].filter(item => item.visible !== false);

  return (
    <Sider width={250} theme="dark">
      <div style={{ 
        height: 64, 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center',
        color: 'white',
        fontSize: 18,
        fontWeight: 'bold',
      }}>
        TikMan OLT
      </div>
      <Menu
        theme="dark"
        mode="inline"
        selectedKeys={[location.pathname]}
        items={menuItems}
      />
    </Sider>
  );
}
```

### Example Page

```typescript
// presentation/pages/UsersPage.tsx
import { useState } from 'react';
import { Button, Space } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { PageHeader } from '../components/common/PageHeader';
import { UserTable } from '../components/features/users/UserTable';
import { UserModal } from '../components/features/users/UserModal';
import { useUsers, useCreateUser, useUpdateUser, useDeleteUser } from '@/application/hooks/users';
import type { User, CreateUserDto, UpdateUserDto } from '@/domain/entities';

export default function UsersPage() {
  const [modalVisible, setModalVisible] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);

  const { data: users, isLoading } = useUsers();
  const createUser = useCreateUser();
  const updateUser = useUpdateUser();
  const deleteUser = useDeleteUser();

  const handleCreate = () => {
    setEditingUser(null);
    setModalVisible(true);
  };

  const handleEdit = (user: User) => {
    setEditingUser(user);
    setModalVisible(true);
  };

  const handleDelete = (id: string) => {
    deleteUser.mutate(id);
  };

  const handleSubmit = async (values: CreateUserDto | UpdateUserDto) => {
    if (editingUser) {
      await updateUser.mutateAsync({ id: editingUser.id, data: values as UpdateUserDto });
    } else {
      await createUser.mutateAsync(values as CreateUserDto);
    }
    setModalVisible(false);
  };

  return (
    <div>
      <PageHeader
        title="Users"
        extra={[
          <Button
            key="add"
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreate}
          >
            Add User
          </Button>,
        ]}
      />

      <UserTable
        users={users || []}
        loading={isLoading}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

      <UserModal
        open={modalVisible}
        user={editingUser}
        onCancel={() => setModalVisible(false)}
        onSubmit={handleSubmit}
      />
    </div>
  );
}
```

## 8. Configuration

### Environment Variables

```bash
# .env.example
VITE_API_URL=http://localhost:8080
VITE_APP_NAME=TikMan OLT Provisioning
VITE_WS_URL=ws://localhost:8080/ws

# .env.development
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws

# .env.production
VITE_API_URL=https://api.tikman.com
VITE_WS_URL=wss://api.tikman.com/ws
```

```typescript
// shared/config/env.ts
export const env = {
  apiUrl: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  appName: import.meta.env.VITE_APP_NAME || 'TikMan',
  wsUrl: import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws',
  isDevelopment: import.meta.env.DEV,
  isProduction: import.meta.env.PROD,
} as const;
```

### React Query Configuration

```typescript
// shared/config/queryClient.ts
import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 minutes
      cacheTime: 10 * 60 * 1000, // 10 minutes
      refetchOnWindowFocus: false,
      retry: 1,
      refetchOnMount: 'always',
    },
    mutations: {
      retry: 0,
    },
  },
});
```

### Ant Design Theme

```typescript
// shared/config/theme.ts
import type { ThemeConfig } from 'antd';

export const theme: ThemeConfig = {
  token: {
    colorPrimary: '#1890ff',
    borderRadius: 6,
    fontSize: 14,
  },
  components: {
    Layout: {
      siderBg: '#001529',
      headerBg: '#fff',
    },
    Menu: {
      darkItemBg: '#001529',
      darkItemSelectedBg: '#1890ff',
    },
  },
};
```

### TypeScript Configuration

```json
// tsconfig.json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,

    /* Bundler mode */
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",

    /* Linting */
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,

    /* Path aliases */
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"],
      "@/domain/*": ["src/domain/*"],
      "@/application/*": ["src/application/*"],
      "@/infrastructure/*": ["src/infrastructure/*"],
      "@/presentation/*": ["src/presentation/*"],
      "@/shared/*": ["src/shared/*"]
    }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

### Vite Configuration

```typescript
// vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react-swc';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@/domain': path.resolve(__dirname, './src/domain'),
      '@/application': path.resolve(__dirname, './src/application'),
      '@/infrastructure': path.resolve(__dirname, './src/infrastructure'),
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
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-antd': ['antd', '@ant-design/icons'],
          'vendor-query': ['@tanstack/react-query'],
        },
      },
    },
    chunkSizeWarningLimit: 1000,
  },
});
```

## 9. Docker & Deployment

### Dockerfile

```dockerfile
# Multi-stage build
FROM node:20-alpine AS builder

WORKDIR /app

# Install dependencies
COPY package.json package-lock.json ./
RUN npm ci

# Build
COPY . .
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy built files
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx config
COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

### Nginx Configuration

```nginx
# nginx.conf
events {
  worker_connections 1024;
}

http {
  include /etc/nginx/mime.types;
  default_type application/octet-stream;

  server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

    # SPA routing
    location / {
      try_files $uri $uri/ /index.html;
    }

    # API proxy
    location /api {
      proxy_pass http://tikman-api:8080;
      proxy_set_header Host $host;
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
      proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket proxy (future)
    location /ws {
      proxy_pass http://tikman-api:8080;
      proxy_http_version 1.1;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";
      proxy_set_header Host $host;
    }

    # Security headers
    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;
    add_header X-XSS-Protection "1; mode=block";
  }
}
```

### Docker Compose Integration

```yaml
# Add to docker-compose.yml
services:
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    container_name: tikman-frontend
    ports:
      - "80:80"
    environment:
      - VITE_API_URL=http://localhost:8080
    depends_on:
      - api
    networks:
      - tikman-network
    restart: unless-stopped
```

## 10. Testing Strategy

### Unit Tests

```typescript
// domain/__tests__/User.test.ts
import { describe, it, expect } from 'vitest';
import { UserRole } from '@/domain/entities';

describe('UserRole', () => {
  it('should have correct enum values', () => {
    expect(UserRole.ADMIN).toBe('admin');
    expect(UserRole.TECHNICIAN).toBe('technician');
    expect(UserRole.VIEWER).toBe('viewer');
  });
});
```

```typescript
// infrastructure/__tests__/UserRepository.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { UserRepository } from '@/infrastructure/repositories/UserRepository';

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
    repository = new UserRepository(mockClient);
  });

  it('should fetch all users', async () => {
    const mockUsers = [{ id: '1', username: 'admin' }];
    mockClient.get.mockResolvedValue({ data: mockUsers });

    const users = await repository.getAll();

    expect(users).toEqual(mockUsers);
    expect(mockClient.get).toHaveBeenCalledWith('/api/v1/users');
  });
});
```

### Component Tests

```typescript
// presentation/__tests__/UserTable.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { UserTable } from '@/presentation/components/features/users/UserTable';
import { UserRole } from '@/domain/entities';

describe('UserTable', () => {
  const mockUsers = [
    {
      id: '1',
      username: 'admin',
      email: 'admin@test.com',
      role: UserRole.ADMIN,
      createdAt: '2024-01-01',
      updatedAt: '2024-01-01',
    },
  ];

  it('should render users', () => {
    render(<UserTable users={mockUsers} onEdit={vi.fn()} onDelete={vi.fn()} />);

    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('admin@test.com')).toBeInTheDocument();
  });

  it('should call onDelete when delete clicked', async () => {
    const onDelete = vi.fn();
    render(<UserTable users={mockUsers} onEdit={vi.fn()} onDelete={onDelete} />);

    const deleteBtn = screen.getByRole('button', { name: /delete/i });
    await userEvent.click(deleteBtn);

    expect(onDelete).toHaveBeenCalledWith('1');
  });
});
```

## 11. File Naming Conventions

**Domain Layer:**
- Entities: `PascalCase.ts` (User.ts, Site.ts)
- Interfaces: `IPascalCase.ts` (IUserRepository.ts)

**Infrastructure Layer:**
- Implementations: `PascalCase.ts` (UserRepository.ts)
- Config: `camelCase.ts` (apiClient.ts)

**Application Layer:**
- Hooks: `useCamelCase.ts` (useUsers.ts)
- Contexts: `PascalCaseContext.tsx` (AuthContext.tsx)
- Stores: `camelCaseStore.ts` (authStore.ts)

**Presentation Layer:**
- Components: `PascalCase/index.tsx` (UserTable/index.tsx)
- Pages: `PascalCasePage.tsx` (UsersPage.tsx)
- Styles: `styles.module.css`

**Tests:**
- Same name + `.test.ts(x)` (UserRepository.test.ts)

## 12. Code Quality Standards

### ESLint Configuration

```javascript
// eslint.config.js
import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['dist'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2020,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],
    },
  },
);
```

### Prettier Configuration

```javascript
// prettier.config.js
export default {
  semi: true,
  singleQuote: true,
  tabWidth: 2,
  trailingComma: 'es5',
  printWidth: 100,
  arrowParens: 'always',
};
```

## 13. Performance Targets

**Bundle Size:**
- Initial load: < 200KB gzipped
- Route chunks: < 100KB each
- Total JS: < 500KB gzipped

**Performance Metrics:**
- First Contentful Paint (FCP): < 1.5s
- Largest Contentful Paint (LCP): < 2.5s
- Time to Interactive (TTI): < 3.5s
- Cumulative Layout Shift (CLS): < 0.1

**Optimization Techniques:**
- Code splitting by route
- React Query caching (5min stale time)
- Memoization (useMemo, useCallback)
- Lazy loading images
- Debounced search inputs

## 14. Browser Support

- Chrome: last 2 versions
- Firefox: last 2 versions
- Safari: last 2 versions
- Edge: last 2 versions
- Mobile: iOS Safari 14+, Chrome Android

## 15. Accessibility (WCAG 2.1 Level AA)

- Keyboard navigation support
- ARIA labels on interactive elements
- Color contrast ratios ≥ 4.5:1
- Focus indicators visible
- Screen reader friendly
- Error messages accessible

## 16. Implementation Phases

### Phase 1: Foundation (Week 1)
- Project scaffolding
- Domain layer (entities, interfaces)
- Infrastructure layer (API client, repositories)
- Application layer (hooks, contexts)

### Phase 2: Authentication & Layout (Week 1)
- Auth flow (login/logout)
- Protected routes
- App layout (sidebar, header)
- Dashboard page (empty state)

### Phase 3: User Management (Week 2)
- Users page
- User table component
- User form & modal
- CRUD operations

### Phase 4: Site & OLT Management (Week 2)
- Sites page & components
- OLTs page & components
- Full CRUD for both

### Phase 5: Polish & Testing (Week 3)
- Error handling
- Loading states
- Empty states
- Unit & component tests
- E2E critical paths

### Phase 6: Deployment (Week 3)
- Docker setup
- Nginx configuration
- Environment configs
- Production build optimization

## 17. Success Criteria

✅ **Functional:**
- Users can login with session auth
- Role-based access enforced (Admin/Technician/Viewer)
- Full CRUD for Users, Sites, OLTs
- Real-time data updates via React Query
- Responsive on desktop & tablet

✅ **Technical:**
- Clean Architecture followed
- TypeScript strict mode, no `any`
- Test coverage: Domain 100%, Application 80%, Components 70%
- Bundle size under targets
- Performance metrics met

✅ **Quality:**
- No console errors/warnings
- Proper error handling & user feedback
- Accessible (keyboard nav, screen readers)
- Code passes ESLint & Prettier
- Documentation complete

---

**End of Frontend Design Specification**
