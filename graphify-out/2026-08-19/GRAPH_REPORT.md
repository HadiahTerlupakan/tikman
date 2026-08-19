# Graph Report - tikman  (2026-08-19)

## Corpus Check
- 291 files · ~133,152 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1979 nodes · 3844 edges · 161 communities (141 shown, 20 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 451 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `beaf4d4d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- OLTService
- Olt
- User
- DelegationManager
- common/index.ts
- time.Time
- Ont
- devDependencies
- dependencies
- testing.T
- compilerOptions
- useONTs.ts
- github.com/google/uuid.UUID
- SiteService
- connectivity_test.go
- lookupByPortAndONT
- AlertRule
- collectMetrics
- Topology Discovery Metrics Fix
- ONT Metrics Display Fix - Documentation
- Setup
- OLT Validation and Connection Test Design
- compilerOptions
- App.tsx
- gorm.io/gorm.DB
- Sites.tsx
- ⚠️ Security Concerns & Recommendations
- StatsCard.tsx
- test_snmp_olt_port23161.go
- vite-env.d.ts
- FIX_DUPLICATE_SERIALS.sh
- setup-ont-monitoring.sh
- test-topology.sh
- github.com/tikman/olt-provisioning
- Security Policy
- 🔒 Security Improvements Summary - TikMan
- 🎯 ONT Monitoring Fix - Summary Report
- Global Constraints
- entities/index.ts
- 🚀 ONT Monitoring Quick Start Guide
- 🚀 INSTRUCTIONS TO FIX YOUR ONT MONITORING
- index.tsx
- TikMan - ZTE OLT Provisioning System
- WalkONTMetrics
- 🔧 Troubleshooting: Duplicate ONT Serial Numbers
- NewUserService
- File Structure
- Code Quality Refactoring Report - TikMan
- kdco-primitives/index.ts
- Global Constraints
- setupAuthTest
- RateLimitMiddleware
- Source of Truth (SOT) - TikMan
- Manual Testing - OLT Validation
- MetricsService
- CLAUDE.md
- UserService
- TikMan OLT Provisioning - Frontend Application Design Specification
- 4. API Endpoints
- Monitoring Module Phase 2: Metrics Collection Implementation Plan
- ONT List Performance Optimization
- test_traffic_stats.sh script
- Traffic Statistics Implementation - Complete Summary
- 3. Database Schema
- TikMan Frontend
- Monitoring Module Design Document
- 10. Implementation Phases
- 16. Implementation Phases
- 6. Frontend Structure
- ToOLTResponse
- Traffic Statistics Testing Guide
- 14. Design Decisions (Resolved)
- 5. API Endpoints
- 8. Configuration
- ZTE OLT Provisioning Application - Design Specification
- 8. Security
- dto.go
- NewStore
- Backend Modules
- 11. Testing Strategy
- 8. Frontend Components
- 9. Technology Stack
- 10. Data Flow Examples
- 11. Deployment
- 12. Testing Strategy
- 5. Worker Service & Job Types
- 7. Technology Stack
- 9. Error Handling
- AuthMiddleware
- setupRBACRouter
- AI-Generated Code Standards
- 4. Database Schema
- 6. ZTE OLT Integration
- 6. Application Layer
- 7. Presentation Layer
- 9. Docker & Deployment
- 13. Future Enhancements (Out of Scope for MVP)
- 14. Success Criteria
- 2. System Architecture
- 2. Requirements
- 3. Architecture
- 7. Worker Implementation
- 10. Testing Strategy
- 12. Code Quality Standards
- 1. Overview
- 4. Domain Layer
- 5. Infrastructure Layer
- scripts
- .opencode/package.json
- Frontend Modules
- Store
- frontend/package.json
- Design Decisions Log
- DiscoverOLTTopology
- NewOLTValidatorService
- Code Quality Standards
- Testing Notes
- Common Patterns
- Architecture
- Development Commands
- eslint-plugin-react-refresh
- jsdom
- @testing-library/jest-dom
- @testing-library/react
- @types/humps
- @types/node
- typescript
- vite
- @vitejs/plugin-react-swc
- github.com/gin-gonic/gin.Context
- errorMapper.ts
- MonitoringWorker
- LineProfile
- New
- github.com/gin-gonic/gin.HandlerFunc
- main
- snmp_parsing.go
- ONTLocation
- ONTMetrics
- DiscoveredONT
- NotificationSettings

## God Nodes (most connected - your core abstractions)
1. `setupTestDB()` - 100 edges
2. `NewONTService()` - 86 edges
3. `NewSiteService()` - 79 edges
4. `DelegationManager` - 55 edges
5. `NewOLTService()` - 53 edges
6. `NewEventService()` - 50 edges
7. `NewMetricsService()` - 39 edges
8. `createTestOLT()` - 30 edges
9. `createTestONT()` - 30 edges
10. `Setup()` - 27 edges

## Surprising Connections (you probably didn't know these)
- `Setup()` --calls--> `NewAuthHandler()`  [INFERRED]
  backend/internal/api/router.go → backend/internal/api/auth_handler.go
- `SetupAuthHandlerTest()` --calls--> `NewAuthHandler()`  [INFERRED]
  backend/internal/api/test_helpers.go → backend/internal/api/auth_handler.go
- `Setup()` --calls--> `NewEventHandler()`  [INFERRED]
  backend/internal/api/router.go → backend/internal/api/event_handler.go
- `Setup()` --calls--> `NewMetricsHandler()`  [INFERRED]
  backend/internal/api/router.go → backend/internal/api/metrics_handler.go
- `Setup()` --calls--> `NewOLTHandler()`  [INFERRED]
  backend/internal/api/router.go → backend/internal/api/olt_handler.go

## Import Cycles
- None detected.

## Communities (161 total, 20 thin omitted)

### Community 0 - "OLTService"
Cohesion: 0.12
Nodes (9): SeedEventsResponse, SeedHandler, autoDiscoverONTMetrics(), OLTHandler, NewOLTHandler(), NewSeedHandler(), OLT, OLTService (+1 more)

### Community 1 - "Olt"
Cohesion: 0.11
Nodes (18): oltRepository, useCreateOlt(), useDeleteOlt(), useOlts(), useUpdateOlt(), CreateOltDto, Olt, OltProtocol (+10 more)

### Community 2 - "User"
Cohesion: 0.11
Nodes (19): useCreateUser(), useDeleteUser(), userRepository, useUpdateUser(), useUsers(), AuthState, createMockUser(), createWrapper() (+11 more)

### Community 3 - "DelegationManager"
Cohesion: 0.05
Nodes (35): AssistantSessionMessageItem, BackgroundAgentsPlugin(), BackgroundAgentsPluginWithInternals, createDelegate(), createDelegationList(), createDelegationRead(), createLogger(), DelegateArgs (+27 more)

### Community 4 - "common/index.ts"
Cohesion: 0.29
Nodes (4): DarkCard(), DarkCardProps, PageHeader(), PageHeaderProps

### Community 5 - "time.Time"
Cohesion: 0.12
Nodes (9): EventHandler, NewEventHandler(), ONTEvent, EventService, gorm.io/datatypes.JSON, time.Time, AuditLog, ONT (+1 more)

### Community 6 - "Ont"
Cohesion: 0.05
Nodes (36): ontRepository, useOntAvailability(), useOntEvents(), GponPortEntity, GPONSlot, useOntListLogic(), ontRepository, useOntMetrics() (+28 more)

### Community 7 - "devDependencies"
Cohesion: 0.10
Nodes (21): eslint, eslint-plugin-react-hooks, devDependencies, eslint, eslint-plugin-react-hooks, prettier, @testing-library/user-event, @types/react (+13 more)

### Community 8 - "dependencies"
Cohesion: 0.09
Nodes (23): @ant-design/icons, @ant-design/pro-components, @ant-design/pro-layout, antd, axios, dependencies, @ant-design/icons, @ant-design/pro-components (+15 more)

### Community 9 - "testing.T"
Cohesion: 0.06
Nodes (152): TestSiteHandler_Create(), TestSiteHandler_Delete(), TestSiteHandler_GetByID(), TestSiteHandler_List(), TestSiteHandler_Update(), SetupSiteHandlerTest(), TestQueryONTMetrics(), NewEventService() (+144 more)

### Community 10 - "compilerOptions"
Cohesion: 0.06
Nodes (34): compilerOptions, allowImportingTsExtensions, baseUrl, isolatedModules, jsx, lib, module, moduleResolution (+26 more)

### Community 11 - "useONTs.ts"
Cohesion: 0.16
Nodes (9): CreateONTPayload, ONT, ONTStatus, UpdateONTPayload, ONTListParams, ONTListResponse, ONTRepository, ontRepository (+1 more)

### Community 12 - "github.com/google/uuid.UUID"
Cohesion: 0.16
Nodes (10): CreateONTRequest, ONTResponse, UpdateONTRequest, ToONTResponse(), ToONTResponseWithMetrics(), ONT, ONTStatus, ONTService (+2 more)

### Community 14 - "connectivity_test.go"
Cohesion: 0.14
Nodes (13): TestPingTest_Success(), TestPingTest_Timeout(), TestSNMPTest_InvalidHost(), TestSNMPTest_InvalidPort(), TestSSHTest_InvalidHost(), TestSSHTest_InvalidPort(), TestTelnetTest_InvalidHost(), TestTelnetTest_InvalidPort() (+5 more)

### Community 15 - "lookupByPortAndONT"
Cohesion: 0.17
Nodes (10): DecodeZxGponIfIndex(), EncodeZxGponIfIndex(), StatusMap(), TestDecodeZxGponIfIndex(), TestEncodeDecodeRoundTrip(), TestEncodeZxGponIfIndex(), TestStatusMap(), T (+2 more)

### Community 16 - "AlertRule"
Cohesion: 0.20
Nodes (7): Alarm, AlarmSeverity, AlarmStatus, AlarmType, AlertCondition, AlertMetricType, AlertRule

### Community 17 - "collectMetrics"
Cohesion: 0.22
Nodes (9): main(), collectMetrics(), main(), mapRunStateToStatus(), Config, Load(), validateConfig(), Connect() (+1 more)

### Community 18 - "Topology Discovery Metrics Fix"
Cohesion: 0.06
Nodes (30): 1. Backend - Gunakan WalkONTMetrics (backend/internal/connectivity/snmp.go), 1. Restart Services, 2. Frontend - Fix Snake Case Conversion (frontend/src/presentation/pages/OntListPage.tsx), 2. Test Topology Discovery, 3. Verifikasi Hasil, 4. Check Backend Logs, Cara Testing, Catatan Penting (+22 more)

### Community 19 - "ONT Metrics Display Fix - Documentation"
Cohesion: 0.08
Nodes (23): 1. Backend API - DTO (backend/internal/api/dto.go), 1. Verifikasi Backend Build, 2. Backend Handler (backend/internal/api/ont_handler.go), 2. Verifikasi Monitoring Worker, 3. Backend Router (backend/internal/api/router.go), 3. Verifikasi API Response (dengan authentication), 4. Frontend Entity (frontend/src/domain/entities/Ont.ts), 4. Verifikasi Frontend Display (+15 more)

### Community 20 - "Setup"
Cohesion: 0.16
Nodes (12): NewONTHandler(), Setup(), TestHealthEndpoint(), TestRouterSetup(), NewSiteHandler(), NewMemoryStore(), AuditService, NewAuditService() (+4 more)

### Community 21 - "OLT Validation and Connection Test Design"
Cohesion: 0.06
Nodes (33): Architecture, Component Overview, Data Flow, Dependencies, Error Handling, Error Scenarios, Failure Flow (Example: SNMP timeout), Future Enhancements (Out of Scope) (+25 more)

### Community 22 - "compilerOptions"
Cohesion: 0.22
Nodes (8): compilerOptions, allowSyntheticDefaultImports, composite, module, moduleResolution, skipLibCheck, include, vite.config.ts

### Community 23 - "App.tsx"
Cohesion: 0.36
Nodes (4): App(), router, queryClient, theme

### Community 24 - "gorm.io/gorm.DB"
Cohesion: 0.22
Nodes (15): TestOLTHandler_Create(), TestOLTHandler_Delete(), TestOLTHandler_GetByID(), TestOLTHandler_List(), TestOLTHandler_Update(), CreateTestOLT(), SetupAuthHandlerTest(), SetupOLTHandlerTest() (+7 more)

### Community 25 - "Sites.tsx"
Cohesion: 0.13
Nodes (15): siteRepository, useCreateSite(), useDeleteSite(), useSites(), useUpdateSite(), CreateSiteDto, Site, UpdateSiteDto (+7 more)

### Community 26 - "⚠️ Security Concerns & Recommendations"
Cohesion: 0.06
Nodes (32): 10. 🟡 Default Admin Credentials, 1. Authentication & Authorization ✅, 1. 🟡 Production Environment Variables, 2. 🟡 CORS for Production, 2. Password Security ✅, 3. Data Encryption ✅, 3. 🟡 HTTPS Enforcement, 4. 🟡 Input Validation (+24 more)

### Community 51 - "Security Policy"
Cohesion: 0.06
Nodes (31): Acknowledgments, Additional Resources, Authentication & Authorization, CI/CD Security Checks, Contact, CORS Configuration, Data Protection, Default Credentials (+23 more)

### Community 52 - "🔒 Security Improvements Summary - TikMan"
Cohesion: 0.07
Nodes (28): 1. ✅ Security Headers Middleware, 2. ✅ Environment-Based Configuration, 3. ✅ Enhanced Input Validation, 4. ✅ Production-Ready Router, 5. ✅ Comprehensive Security Documentation, 6. ✅ Updated Environment Configuration, After (9.5/10) 🎉, After Security Improvements (+20 more)

### Community 53 - "🎯 ONT Monitoring Fix - Summary Report"
Cohesion: 0.07
Nodes (26): 1. Dynamic Configuration per OLT, 2. Database Schema Update, 3. UI Configuration Form, 📝 Additional Resources Created, Backend (7 files), ✨ Bonus Features, 🚀 Deployment Steps, 🎯 Expected Results (+18 more)

### Community 54 - "Global Constraints"
Cohesion: 0.08
Nodes (25): Build for production, Build image, Global Constraints, Install dependencies, Plan Complete, Preview production build, Run container, Run dev server (+17 more)

### Community 55 - "entities/index.ts"
Cohesion: 0.17
Nodes (8): IAuthRepository, LoginCredentials, LoginResponse, apiClient, API_ENDPOINTS, AuthRepository, MockClient, env

### Community 56 - "🚀 ONT Monitoring Quick Start Guide"
Cohesion: 0.08
Nodes (25): 📚 Additional Resources, 🛠️ Advanced: Batch Update Existing OLTs, Check Metrics Collection, 🎯 Common Scenarios, Example Configuration, Issue: Invalid Rx/Tx Power values, Issue: No metrics appearing, Issue: Wrong SNMPPort configured (+17 more)

### Community 57 - "🚀 INSTRUCTIONS TO FIX YOUR ONT MONITORING"
Cohesion: 0.08
Nodes (24): 📝 Additional Notes, After Fix (Expected):, Before Fix (Current Problem):, Check Dashboard Metrics, Check Worker Logs, Common ZTE C300 Formats, 📊 Expected Results, For Other OLTs (+16 more)

### Community 58 - "index.tsx"
Cohesion: 0.21
Nodes (12): authRepository, useCurrentUser(), useLogin(), useLogout(), useAuthStore, AppLayout(), Header(), Sidebar() (+4 more)

### Community 59 - "TikMan - ZTE OLT Provisioning System"
Cohesion: 0.09
Nodes (22): API Endpoints, Authentication, Code Formatting, Default Credentials, Development, Development Setup, Features, License (+14 more)

### Community 60 - "WalkONTMetrics"
Cohesion: 0.25
Nodes (6): main(), decodeOnuIDIfIndex(), T, walkONTMetricTable(), WalkONTMetrics(), toInt64()

### Community 61 - "🔧 Troubleshooting: Duplicate ONT Serial Numbers"
Cohesion: 0.09
Nodes (21): 1. Check Worker Logs, 2. Check Dashboard, 3. Verify Metrics Values, Fix Implementation, How It Happens, Method A: Via Web Dashboard (EASIEST), Method B: From Hardware Labels (MOST RELIABLE), Method C: Network Discovery via SNMP (+13 more)

### Community 62 - "NewUserService"
Cohesion: 0.23
Nodes (11): CreateDefaultAdmin(), setupSeedTest(), TestCreateDefaultAdmin_NoUsers(), TestCreateDefaultAdmin_UsersExist(), NewUserService(), TestUserService_Create(), TestUserService_Delete(), TestUserService_GetByUsername() (+3 more)

### Community 63 - "File Structure"
Cohesion: 0.11
Nodes (18): Execution Notes, File Structure, Global Constraints, OLT Validation and Connection Test Implementation Plan, Self-Review Checklist, Task 10: Update Dependency Injection in Main, Task 11: Update OLTService Unit Tests, Task 12: Integration Testing and Documentation (+10 more)

### Community 64 - "Code Quality Refactoring Report - TikMan"
Cohesion: 0.11
Nodes (17): 1. Updated CLAUDE.md ✅, 2. Created test_helpers.go ✅, 3. Refactored Test Files ✅, 4. Fixed Handler Nil Checks ✅, Architecture Quality, Changes Made, Code Quality Metrics, Code Quality Refactoring Report - TikMan (+9 more)

### Community 65 - "kdco-primitives/index.ts"
Cohesion: 0.12
Nodes (19): canUseCmuxWorkflow(), CmuxContext, CmuxEnvironment, detectCmuxContext(), normalizeCmuxValue(), ResolveExecutable, getProjectId(), hashPath() (+11 more)

### Community 66 - "Global Constraints"
Cohesion: 0.12
Nodes (15): Global Constraints, Task 10: Site Service & Handlers, Task 11: OLT Service & Handlers, Task 12: Server Setup with Routes, Task 13: Docker Compose Setup, Task 1: Project Scaffolding & Dependencies, Task 2: Database Connection & Logger Setup, Task 3: Database Models (+7 more)

### Community 67 - "setupAuthTest"
Cohesion: 0.27
Nodes (10): AuthHandler, NewAuthHandler(), setupAuthTest(), TestAuthHandler_Login_InvalidCredentials(), TestAuthHandler_Login_Success(), TestAuthHandler_Login_UserNotFound(), TestAuthHandler_Logout(), TestAuthHandler_Me_NoUserContext() (+2 more)

### Community 68 - "RateLimitMiddleware"
Cohesion: 0.31
Nodes (7): newRateLimiter(), RateLimitMiddleware(), TestRateLimitMiddleware(), TestRateLimitMiddleware_100Requests(), TestRateLimitMiddleware_DifferentIPs(), sync.Mutex, rateLimiter

### Community 69 - "Source of Truth (SOT) - TikMan"
Cohesion: 0.15
Nodes (13): API Contract, Architecture Principles, Change Log, CI/CD Pipeline, Database Schema, Deployment Architecture, Future Roadmap, Maintenance Notes (+5 more)

### Community 70 - "Manual Testing - OLT Validation"
Cohesion: 0.17
Nodes (11): Expected Response Format, Known Test Behavior, Linting, Manual Testing - OLT Validation, Run Specific Test, Running Tests, Test Scenarios, Test with Mock OLT (+3 more)

### Community 71 - "MetricsService"
Cohesion: 0.15
Nodes (7): MetricsHandler, ONTMetricsResponse, ToONTMetricsResponse(), NewMetricsHandler(), formatPowerValue(), MetricsService, ONTMetricsRow

### Community 72 - "CLAUDE.md"
Cohesion: 0.18
Nodes (8): CI/CD Pipeline, Default Credentials, Deployment Secrets Required, Environment Variables, GitHub Actions Workflows, graphify, Project Overview, Security Notes

### Community 73 - "UserService"
Cohesion: 0.14
Nodes (11): User, UserService, ComparePassword(), Decrypt(), Encrypt(), HashPassword(), TestComparePassword_Invalid(), TestComparePassword_Valid() (+3 more)

### Community 74 - "TikMan OLT Provisioning - Frontend Application Design Specification"
Cohesion: 0.20
Nodes (9): 11. File Naming Conventions, 13. Performance Targets, 14. Browser Support, 15. Accessibility (WCAG 2.1 Level AA), 17. Success Criteria, 2. Clean Architecture Principles, 3. Project Structure, Architecture Layers (+1 more)

### Community 75 - "4. API Endpoints"
Cohesion: 0.20
Nodes (10): 4. API Endpoints, Audit Logs, Authentication, Dashboard & Monitoring, Health Check, OLTs, ONTs, Service & Line Profiles (+2 more)

### Community 76 - "Monitoring Module Phase 2: Metrics Collection Implementation Plan"
Cohesion: 0.22
Nodes (8): Final Verification, Global Constraints, Monitoring Module Phase 2: Metrics Collection Implementation Plan, Task 1: SNMP Metrics Query Implementation, Task 2: Metrics Storage Service, Task 3: Metrics Collection Worker, Task 4: Metrics API Endpoints, Task 5: Frontend Metrics Display

### Community 77 - "ONT List Performance Optimization"
Cohesion: 0.20
Nodes (9): Deployment Notes, Files Changed, Implementation, ONT List Performance Optimization, Performance Results, Problem, Solution, Testing (+1 more)

### Community 79 - "Traffic Statistics Implementation - Complete Summary"
Cohesion: 0.06
Nodes (31): 1. Apply Database Migration, 1. Backend Data Structures, 2. Restart Services, 2. SNMP Data Collection, 3. Topology Discovery Update, 3. Wait for Data Collection, 4. Database Storage, 4. Verify Database (+23 more)

### Community 80 - "3. Database Schema"
Cohesion: 0.25
Nodes (8): 3. Database Schema, audit_logs, line_profiles, olts, onts, service_profiles, sites, users

### Community 81 - "TikMan Frontend"
Cohesion: 0.25
Nodes (7): Architecture, Development, Docker, Environment Variables, Project Structure, Tech Stack, TikMan Frontend

### Community 83 - "Monitoring Module Design Document"
Cohesion: 0.29
Nodes (6): 12. Security Considerations, 13. Monitoring & Observability, 15. Success Criteria, 16. Risks & Mitigations, 1. Overview, Monitoring Module Design Document

### Community 84 - "10. Implementation Phases"
Cohesion: 0.29
Nodes (7): 10. Implementation Phases, Phase 1: Foundation (Week 1), Phase 2: Metrics Collection (Week 2), Phase 3: Visualization (Week 3), Phase 4: Alerting (Week 4), Phase 5: Optimization & Testing (Week 5), Phase 6: Mobile App (Future - Month 2+)

### Community 85 - "16. Implementation Phases"
Cohesion: 0.29
Nodes (7): 16. Implementation Phases, Phase 1: Foundation (Week 1), Phase 2: Authentication & Layout (Week 1), Phase 3: User Management (Week 2), Phase 4: Site & OLT Management (Week 2), Phase 5: Polish & Testing (Week 3), Phase 6: Deployment (Week 3)

### Community 86 - "6. Frontend Structure"
Cohesion: 0.29
Nodes (7): 6. Frontend Structure, Dashboard Components, ONT Discovery Workflow, ONTs Management Features, Pages & Routes, Real-time Updates, Tech Stack

### Community 87 - "ToOLTResponse"
Cohesion: 0.29
Nodes (4): ToOLTResponse(), OLTHandler, OLTHandler, OLTHandler

### Community 88 - "Traffic Statistics Testing Guide"
Cohesion: 0.11
Nodes (17): Expected Output, Failure Case - No Traffic Data:, Issue: "No ONTs have traffic statistics", Issue: "SNMP timeout", Issue: "Target ONT not found", Option 1: Automated Test Script, Option 2: Manual Test, Overview (+9 more)

### Community 89 - "14. Design Decisions (Resolved)"
Cohesion: 0.33
Nodes (6): 14.1 Polling Frequency ✅, 14.2 Webhook Format ✅, 14.3 Email Templates ✅, 14.4 Mobile App ✅, 14.5 Multi-Tenancy ✅, 14. Design Decisions (Resolved)

### Community 90 - "5. API Endpoints"
Cohesion: 0.33
Nodes (6): 5.1 ONT Management, 5.2 Monitoring Queries, 5.3 Alarms, 5.4 Alert Rules, 5.5 Notifications, 5. API Endpoints

### Community 91 - "8. Configuration"
Cohesion: 0.33
Nodes (6): 8. Configuration, Ant Design Theme, Environment Variables, React Query Configuration, TypeScript Configuration, Vite Configuration

### Community 92 - "ZTE OLT Provisioning Application - Design Specification"
Cohesion: 0.33
Nodes (5): 15. Project Structure, 16. Glossary, 1. Overview, Key Features, ZTE OLT Provisioning Application - Design Specification

### Community 93 - "8. Security"
Cohesion: 0.33
Nodes (6): 8. Security, API Security, Authentication & Authorization, Credential Storage, Logging & Audit, Network Security

### Community 94 - "dto.go"
Cohesion: 0.13
Nodes (17): CreateOLTRequest, CreateSiteRequest, CreateUserRequest, ErrorResponse, LoginRequest, LoginResponse, OLTResponse, SiteResponse (+9 more)

### Community 95 - "NewStore"
Cohesion: 0.50
Nodes (7): NewStore(), redis.Client, setupTestRedis(), TestSessionStore_CreateAndGet(), TestSessionStore_Delete(), TestSessionStore_GetInvalidToken(), TestSessionStore_Refresh()

### Community 96 - "Backend Modules"
Cohesion: 0.18
Nodes (11): Backend Modules, `cmd/api/`, `internal/api/`, `internal/auth/`, `internal/config/`, `internal/database/`, `internal/logger/`, `internal/middleware/` (+3 more)

### Community 97 - "11. Testing Strategy"
Cohesion: 0.40
Nodes (5): 11.1 Unit Tests, 11.2 Integration Tests, 11.3 Performance Tests, 11.4 Manual Testing, 11. Testing Strategy

### Community 98 - "8. Frontend Components"
Cohesion: 0.40
Nodes (5): 8.1 Dashboard Page, 8.2 ONT Detail Page, 8.3 Alarms Page, 8.4 Alert Rules Page, 8. Frontend Components

### Community 99 - "9. Technology Stack"
Cohesion: 0.40
Nodes (5): 9. Technology Stack, Backend, Database, Frontend, Infrastructure

### Community 100 - "10. Data Flow Examples"
Cohesion: 0.40
Nodes (5): 10. Data Flow Examples, Auto-Discovery Flow, Monitoring/Polling Flow, ONT Provisioning Flow, WebSocket Message Types

### Community 101 - "11. Deployment"
Cohesion: 0.40
Nodes (5): 11. Deployment, Docker Compose Structure, Environment Variables (.env), GitHub Actions CI/CD, Initial Setup

### Community 102 - "12. Testing Strategy"
Cohesion: 0.40
Nodes (5): 12. Testing Strategy, Backend Testing, Development Workflow, Frontend Testing, Manual Testing Priority

### Community 103 - "5. Worker Service & Job Types"
Cohesion: 0.40
Nodes (5): 5. Worker Service & Job Types, Job Queue Architecture, Job Types, WebSocket Broadcast, Worker Configuration

### Community 104 - "7. Technology Stack"
Cohesion: 0.40
Nodes (5): 7. Technology Stack, Backend (Go), Development Tools, Frontend (React), Infrastructure

### Community 105 - "9. Error Handling"
Cohesion: 0.40
Nodes (5): 9. Error Handling, Backend Error Responses, Frontend Error Handling, Graceful Degradation, Worker Error Handling

### Community 106 - "AuthMiddleware"
Cohesion: 0.54
Nodes (7): AuthMiddleware(), redis.Client, setupTestRedis(), setupTestRouter(), TestAuthMiddleware_InvalidToken(), TestAuthMiddleware_NoToken(), TestAuthMiddleware_ValidToken()

### Community 107 - "setupRBACRouter"
Cohesion: 0.43
Nodes (7): setupRBACRouter(), TestRequireRole_Allowed(), TestRequireRole_Forbidden(), TestRequireRole_NoRole(), TestRequireRole_SingleRoleMatch(), TestRequireRole_TechnicianAllowed(), github.com/gin-gonic/gin.Engine

### Community 108 - "AI-Generated Code Standards"
Cohesion: 0.25
Nodes (8): AI-Generated Code Standards, AI Slop Review Checklist, Comments, Documentation & Reporting, Follow What Exists, No Leftovers, Scope Discipline, Tests, Not Test Theatre

### Community 109 - "4. Database Schema"
Cohesion: 0.50
Nodes (4): 4.1 PostgreSQL (Metadata & Alarms), 4.2 TimescaleDB (Time-Series Metrics), 4.3 Redis Cache Structure, 4. Database Schema

### Community 110 - "6. ZTE OLT Integration"
Cohesion: 0.50
Nodes (4): 6.1 SNMP OIDs (Primary Method), 6.2 Telnet Commands (Fallback/Detail), 6.3 Polling Strategy, 6. ZTE OLT Integration

### Community 111 - "6. Application Layer"
Cohesion: 0.50
Nodes (4): 6. Application Layer, Auth Store (Zustand), Custom Hooks, Repository Context (DI)

### Community 112 - "7. Presentation Layer"
Cohesion: 0.50
Nodes (4): 7. Presentation Layer, Example Page, Layout Components, Routes

### Community 113 - "9. Docker & Deployment"
Cohesion: 0.50
Nodes (4): 9. Docker & Deployment, Docker Compose Integration, Dockerfile, Nginx Configuration

### Community 114 - "13. Future Enhancements (Out of Scope for MVP)"
Cohesion: 0.50
Nodes (4): 13. Future Enhancements (Out of Scope for MVP), Phase 2 Features, Scalability Improvements, Security Enhancements

### Community 115 - "14. Success Criteria"
Cohesion: 0.50
Nodes (4): 14. Success Criteria, MVP Acceptance, Performance Targets, Reliability Targets

### Community 116 - "2. System Architecture"
Cohesion: 0.50
Nodes (4): 2. System Architecture, Architecture Pattern, Communication Flow, Components

### Community 117 - "2. Requirements"
Cohesion: 0.67
Nodes (3): 2.1 Functional Requirements, 2.2 Non-Functional Requirements, 2. Requirements

### Community 118 - "3. Architecture"
Cohesion: 0.67
Nodes (3): 3.1 System Components, 3.2 Data Flow, 3. Architecture

### Community 119 - "7. Worker Implementation"
Cohesion: 0.67
Nodes (3): 7.1 Worker Pool Architecture, 7.2 Scheduler, 7. Worker Implementation

### Community 120 - "10. Testing Strategy"
Cohesion: 0.67
Nodes (3): 10. Testing Strategy, Component Tests, Unit Tests

### Community 121 - "12. Code Quality Standards"
Cohesion: 0.67
Nodes (3): 12. Code Quality Standards, ESLint Configuration, Prettier Configuration

### Community 122 - "1. Overview"
Cohesion: 0.67
Nodes (3): 1. Overview, Key Features, Technology Stack

### Community 123 - "4. Domain Layer"
Cohesion: 0.67
Nodes (3): 4. Domain Layer, Entities, Repository Interfaces

### Community 124 - "5. Infrastructure Layer"
Cohesion: 0.67
Nodes (3): 5. Infrastructure Layer, API Client, Repository Implementations

### Community 125 - "scripts"
Cohesion: 0.25
Nodes (8): scripts, build, dev, format, lint, preview, test, test:ui

### Community 126 - ".opencode/package.json"
Cohesion: 0.20
Nodes (9): @opencode-ai/plugin, dependencies, @opencode-ai/plugin, devDependencies, unique-names-generator, name, private, type (+1 more)

### Community 127 - "Frontend Modules"
Cohesion: 0.29
Nodes (7): Frontend Modules, Module Inventory, `src/application/`, `src/domain/`, `src/infrastructure/`, `src/presentation/`, `src/shared/`

### Community 128 - "Store"
Cohesion: 0.17
Nodes (6): Data, MemoryStore, Store, redis.Client, sync.RWMutex, time.Duration

### Community 129 - "frontend/package.json"
Cohesion: 0.33
Nodes (5): name, overrides, path-to-regexp, type, version

### Community 130 - "Design Decisions Log"
Cohesion: 0.33
Nodes (6): DD-001: No GORM Preload Usage, DD-002: Session-Based Authentication, DD-003: Service Layer Owns All Business Logic, DD-004: Clean Architecture in Frontend, DD-005: AES-256-GCM for Credential Encryption, Design Decisions Log

### Community 131 - "DiscoverOLTTopology"
Cohesion: 0.50
Nodes (7): newSNMPClient(), DiscoverOLTTopology(), decodeOnuTypeIfIndex(), WalkONTStatuses(), WalkONTHardwareVersions(), WalkONTIPAddresses(), WalkONTMACAddresses()

### Community 132 - "NewOLTValidatorService"
Cohesion: 0.50
Nodes (7): NewOLTValidatorService(), setupValidatorTestDB(), TestNewOLTValidatorService(), TestValidateCreate_LocalhostSSH(), TestValidateIPNotDuplicate_Duplicate(), TestValidateIPNotDuplicate_NoDuplicate(), TestValidationResult_StructureExists()

### Community 133 - "Code Quality Standards"
Cohesion: 0.40
Nodes (5): Anti-Patterns to Avoid, Code Quality Standards, Code Review Checklist, File Size Limits, Refactoring Guidelines

### Community 134 - "Testing Notes"
Cohesion: 0.40
Nodes (5): Backend Testing Requirements, CI/CD Testing, Frontend Testing Requirements, Pre-Commit Checklist, Testing Notes

### Community 135 - "Common Patterns"
Cohesion: 0.50
Nodes (4): Adding a New API Endpoint, Adding a New Model, Common Patterns, Frontend Data Fetching

### Community 136 - "Architecture"
Cohesion: 0.50
Nodes (4): Architecture, Backend Structure (Go), Frontend Structure (React + TypeScript), Key Architectural Patterns

### Community 137 - "Development Commands"
Cohesion: 0.50
Nodes (4): Backend (Go), Development Commands, Frontend (React + TypeScript), Infrastructure

### Community 148 - "github.com/gin-gonic/gin.Context"
Cohesion: 0.11
Nodes (13): ONTHandler, SiteHandler, UserHandler, ToSiteResponse(), ToUserResponse(), OLTHandler, mapStatusToRunState(), OLTHandler (+5 more)

### Community 149 - "errorMapper.ts"
Cohesion: 0.29
Nodes (6): ApiError, ApiErrorResponse, mapApiError(), NotFoundError, UnauthorizedError, ValidationError

### Community 150 - "MonitoringWorker"
Cohesion: 0.39
Nodes (4): MonitoringWorker, context.CancelFunc, context.Context, sync.WaitGroup

### Community 152 - "New"
Cohesion: 0.40
Nodes (4): New(), TestNew_InvalidLogLevel(), TestNew_TimestampFormatUTC(), TestNew_ValidLogLevels()

### Community 153 - "github.com/gin-gonic/gin.HandlerFunc"
Cohesion: 0.40
Nodes (4): RequireRole(), HTTPSRedirect(), SecureHeaders(), github.com/gin-gonic/gin.HandlerFunc

### Community 155 - "main"
Cohesion: 0.32
Nodes (8): formatTimeValue(), main(), GetLastOfflineReason(), GetLastOfflineTime(), GetLastOnlineTime(), PollOntStatus(), encodeOnuIDIfIndex(), parseZteHexTimestamp()

### Community 156 - "snmp_parsing.go"
Cohesion: 0.32
Nodes (6): TestParseZxGponSuffix(), cleanString(), decodeZxGponIfIndex(), ExtractSerialNumber(), isValidHex(), parseZxGponSuffix()

### Community 157 - "ONTLocation"
Cohesion: 0.45
Nodes (9): ONTLocation, copyONTAttributes(), queryPortONTAttributes(), walkONUDescriptions(), walkONUDeviceTypes(), walkONUNames(), walkSerialNumbers(), ExtractName() (+1 more)

### Community 158 - "ONTMetrics"
Cohesion: 0.18
Nodes (11): TestDecodeZxGponPower(), TestEncodeZxGponIfIndexMatchesLiveDevice(), TestToInt64(), buildTopologyStructure(), mapPhaseToRunState(), decodeZxGponPower(), encodeZxGponIfIndex(), ONTMetrics (+3 more)

### Community 159 - "DiscoveredONT"
Cohesion: 0.36
Nodes (6): DiscoverONTs(), DiscoveredONT, GPONSlot, findONT(), main(), GponPort

## Knowledge Gaps
- **692 isolated node(s):** `name`, `private`, `type`, `@opencode-ai/plugin`, `unique-names-generator` (+687 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **20 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `createTestONT()` connect `testing.T` to `github.com/google/uuid.UUID`?**
  _High betweenness centrality (0.007) - this node is a cross-community bridge._
- **Why does `NewONTService()` connect `testing.T` to `OLTService`, `github.com/google/uuid.UUID`, `collectMetrics`, `github.com/gin-gonic/gin.Context`, `Setup`, `gorm.io/gorm.DB`, `main`?**
  _High betweenness centrality (0.005) - this node is a cross-community bridge._
- **Why does `createTestOLT()` connect `testing.T` to `OLTService`, `gorm.io/gorm.DB`, `github.com/google/uuid.UUID`?**
  _High betweenness centrality (0.005) - this node is a cross-community bridge._
- **Are the 90 inferred relationships involving `setupTestDB()` (e.g. with `TestEventService_CalculateAvailability_MTBF_MTTR()` and `TestEventService_CalculateAvailability_MultipleTransitions()`) actually correct?**
  _`setupTestDB()` has 90 INFERRED edges - model-reasoned connections that need verification._
- **Are the 61 inferred relationships involving `NewONTService()` (e.g. with `TestEventService_CalculateAvailability_MTBF_MTTR()` and `TestEventService_CalculateAvailability_MultipleTransitions()`) actually correct?**
  _`NewONTService()` has 61 INFERRED edges - model-reasoned connections that need verification._
- **Are the 66 inferred relationships involving `NewSiteService()` (e.g. with `TestEventService_CalculateAvailability_MTBF_MTTR()` and `TestEventService_CalculateAvailability_MultipleTransitions()`) actually correct?**
  _`NewSiteService()` has 66 INFERRED edges - model-reasoned connections that need verification._
- **What connects `name`, `private`, `type` to the rest of the system?**
  _692 weakly-connected nodes found - possible documentation gaps or missing edges._