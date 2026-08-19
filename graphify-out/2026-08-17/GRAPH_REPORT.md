# Graph Report - tikman  (2026-08-17)

## Corpus Check
- 197 files · ~102,296 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1552 nodes · 2613 edges · 130 communities (113 shown, 17 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 134 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `beaf4d4d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- testing.T
- Olt
- hooks.test.ts
- github.com/gin-gonic/gin.Context
- main
- time.Duration
- OntListPage.tsx
- devDependencies
- dependencies
- snmp.go
- compilerOptions
- useONTs.ts
- github.com/google/uuid.UUID
- UserHandler
- gorm.io/gorm.DB
- Setup
- AlertRule
- NewStore
- Topology Discovery Metrics Fix
- ONT Metrics Display Fix - Documentation
- errorMapper.ts
- OLT Validation and Connection Test Design
- compilerOptions
- App.tsx
- NewUserService
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
- UserService
- 🔧 Troubleshooting: Duplicate ONT Serial Numbers
- dto.go
- File Structure
- Code Quality Refactoring Report - TikMan
- Olts.tsx
- Global Constraints
- NewSiteService
- SiteService
- Source of Truth (SOT) - TikMan
- Manual Testing - OLT Validation
- SetupOLTHandlerTest
- CLAUDE.md
- setupRBACRouter
- TikMan OLT Provisioning - Frontend Application Design Specification
- 4. API Endpoints
- Monitoring Module Phase 2: Metrics Collection Implementation Plan
- NewOLTValidatorService
- scripts
- SetupSiteHandlerTest
- 3. Database Schema
- TikMan Frontend
- time.Time
- Monitoring Module Design Document
- 10. Implementation Phases
- 16. Implementation Phases
- 6. Frontend Structure
- setupSiteTestDB
- New
- 14. Design Decisions (Resolved)
- 5. API Endpoints
- 8. Configuration
- ZTE OLT Provisioning Application - Design Specification
- 8. Security
- package.json
- eslint-plugin-react-refresh
- jsdom
- 11. Testing Strategy
- 8. Frontend Components
- 9. Technology Stack
- 10. Data Flow Examples
- 11. Deployment
- 12. Testing Strategy
- 5. Worker Service & Job Types
- 7. Technology Stack
- 9. Error Handling
- @testing-library/jest-dom
- @testing-library/react
- @types/humps
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
- @types/node
- typescript
- vite
- @vitejs/plugin-react-swc
- NotificationSettings

## God Nodes (most connected - your core abstractions)
1. `User` - 26 edges
2. `Setup()` - 24 edges
3. `NewSiteService()` - 23 edges
4. `setupTestDB()` - 19 edges
5. `useAuthStore` - 19 edges
6. `ONTService` - 18 edges
7. `MonitoringWorker` - 18 edges
8. `Olt` - 18 edges
9. `compilerOptions` - 18 edges
10. `TikMan OLT Provisioning - Frontend Application Design Specification` - 18 edges

## Surprising Connections (you probably didn't know these)
- `setupAuthTest()` --calls--> `NewAuthHandler()`  [INFERRED]
  backend/internal/api/auth_handler_test.go → backend/internal/api/auth_handler.go
- `Setup()` --calls--> `NewAuthHandler()`  [INFERRED]
  backend/internal/api/router.go → backend/internal/api/auth_handler.go
- `SetupAuthHandlerTest()` --calls--> `NewAuthHandler()`  [INFERRED]
  backend/internal/api/test_helpers.go → backend/internal/api/auth_handler.go
- `UpdateONTRequest` --references--> `ONTStatus`  [EXTRACTED]
  backend/internal/api/dto.go → backend/internal/models/ont.go
- `Setup()` --calls--> `NewMetricsHandler()`  [INFERRED]
  backend/internal/api/router.go → backend/internal/api/metrics_handler.go

## Import Cycles
- None detected.

## Communities (130 total, 17 thin omitted)

### Community 0 - "testing.T"
Cohesion: 0.28
Nodes (12): setupAuthTest(), TestAuthHandler_Login_InvalidCredentials(), TestAuthHandler_Login_Success(), TestAuthHandler_Login_UserNotFound(), TestAuthHandler_Logout(), TestAuthHandler_Me_NoUserContext(), TestAuthHandler_Me_Success(), TestAuthHandler_Me_UserNotFound() (+4 more)

### Community 1 - "Olt"
Cohesion: 0.13
Nodes (11): CreateOltDto, Olt, OltProtocol, OltStatus, UpdateOltDto, IOltRepository, OltRepository, OltModalProps (+3 more)

### Community 2 - "hooks.test.ts"
Cohesion: 0.11
Nodes (17): useCreateUser(), useDeleteUser(), userRepository, useUpdateUser(), useUsers(), AuthState, CreateUserDto, UpdateUserDto (+9 more)

### Community 3 - "github.com/gin-gonic/gin.Context"
Cohesion: 0.18
Nodes (10): OLTHandler, ONTHandler, ToOLTResponse(), ToONTResponse(), SetupTestContext(), GetUserID(), GetUserRole(), NewMetricsService() (+2 more)

### Community 4 - "main"
Cohesion: 0.16
Nodes (11): main(), collectMetrics(), main(), Config, Load(), validateConfig(), Connect(), TestConnect_InvalidConfig() (+3 more)

### Community 5 - "time.Duration"
Cohesion: 0.09
Nodes (22): TestPingTest_Success(), TestPingTest_Timeout(), TestSNMPTest_InvalidHost(), TestSNMPTest_InvalidPort(), TestSSHTest_InvalidHost(), TestSSHTest_InvalidPort(), TestTelnetTest_InvalidHost(), TestTelnetTest_InvalidPort() (+14 more)

### Community 6 - "OntListPage.tsx"
Cohesion: 0.10
Nodes (19): ontRepository, useOntMetrics(), ontRepository, useCreateOnt(), useDeleteOnt(), useOnts(), useUpdateOnt(), CreateOntDto (+11 more)

### Community 7 - "devDependencies"
Cohesion: 0.10
Nodes (21): eslint, eslint-plugin-react-hooks, devDependencies, eslint, eslint-plugin-react-hooks, prettier, @testing-library/user-event, @types/react (+13 more)

### Community 8 - "dependencies"
Cohesion: 0.09
Nodes (23): @ant-design/icons, @ant-design/pro-components, @ant-design/pro-layout, antd, axios, dependencies, @ant-design/icons, @ant-design/pro-components (+15 more)

### Community 9 - "snmp.go"
Cohesion: 0.07
Nodes (44): TestDecodeZxGponPower(), TestEncodeZxGponIfIndexMatchesLiveDevice(), TestParseZxGponSuffix(), TestToInt64(), decodeZxGponIfIndex(), decodeZxGponPower(), DiscoverOLTTopology(), DiscoverONTs() (+36 more)

### Community 10 - "compilerOptions"
Cohesion: 0.06
Nodes (34): compilerOptions, allowImportingTsExtensions, baseUrl, isolatedModules, jsx, lib, module, moduleResolution (+26 more)

### Community 11 - "useONTs.ts"
Cohesion: 0.16
Nodes (9): CreateONTPayload, ONT, ONTStatus, UpdateONTPayload, ONTListParams, ONTListResponse, ONTRepository, ontRepository (+1 more)

### Community 12 - "github.com/google/uuid.UUID"
Cohesion: 0.20
Nodes (7): CreateONTRequest, ONTResponse, ToONTResponseWithMetrics(), ONT, ONTStatus, ONTService, github.com/google/uuid.UUID

### Community 13 - "UserHandler"
Cohesion: 0.19
Nodes (6): AuthHandler, UserHandler, NewAuthHandler(), ToUserResponse(), isDuplicateError(), NewUserHandler()

### Community 14 - "gorm.io/gorm.DB"
Cohesion: 0.12
Nodes (7): OLT, OLTStatus, OLTService, NewMonitoringWorker(), gorm.io/gorm.DB, LineProfile, ServiceProfile

### Community 15 - "Setup"
Cohesion: 0.23
Nodes (9): NewOLTHandler(), NewONTHandler(), Setup(), AuditService, NewAuditService(), setupTestAuditDB(), TestAuditService_Log(), TestAuditService_LogUpdate() (+1 more)

### Community 16 - "AlertRule"
Cohesion: 0.20
Nodes (7): Alarm, AlarmSeverity, AlarmStatus, AlarmType, AlertCondition, AlertMetricType, AlertRule

### Community 17 - "NewStore"
Cohesion: 0.20
Nodes (17): NewStore(), redis.Client, setupTestRedis(), TestSessionStore_CreateAndGet(), TestSessionStore_Delete(), TestSessionStore_GetInvalidToken(), TestSessionStore_Refresh(), AuthMiddleware() (+9 more)

### Community 18 - "Topology Discovery Metrics Fix"
Cohesion: 0.06
Nodes (30): 1. Backend - Gunakan WalkONTMetrics (backend/internal/connectivity/snmp.go), 1. Restart Services, 2. Frontend - Fix Snake Case Conversion (frontend/src/presentation/pages/OntListPage.tsx), 2. Test Topology Discovery, 3. Verifikasi Hasil, 4. Check Backend Logs, Cara Testing, Catatan Penting (+22 more)

### Community 19 - "ONT Metrics Display Fix - Documentation"
Cohesion: 0.08
Nodes (23): 1. Backend API - DTO (backend/internal/api/dto.go), 1. Verifikasi Backend Build, 2. Backend Handler (backend/internal/api/ont_handler.go), 2. Verifikasi Monitoring Worker, 3. Backend Router (backend/internal/api/router.go), 3. Verifikasi API Response (dengan authentication), 4. Frontend Entity (frontend/src/domain/entities/Ont.ts), 4. Verifikasi Frontend Display (+15 more)

### Community 20 - "errorMapper.ts"
Cohesion: 0.29
Nodes (6): ApiError, ApiErrorResponse, mapApiError(), NotFoundError, UnauthorizedError, ValidationError

### Community 21 - "OLT Validation and Connection Test Design"
Cohesion: 0.06
Nodes (33): Architecture, Component Overview, Data Flow, Dependencies, Error Handling, Error Scenarios, Failure Flow (Example: SNMP timeout), Future Enhancements (Out of Scope) (+25 more)

### Community 22 - "compilerOptions"
Cohesion: 0.22
Nodes (8): compilerOptions, allowSyntheticDefaultImports, composite, module, moduleResolution, skipLibCheck, include, vite.config.ts

### Community 23 - "App.tsx"
Cohesion: 0.36
Nodes (4): App(), router, queryClient, theme

### Community 24 - "NewUserService"
Cohesion: 0.25
Nodes (11): CreateDefaultAdmin(), setupSeedTest(), TestCreateDefaultAdmin_NoUsers(), TestCreateDefaultAdmin_UsersExist(), NewUserService(), TestUserService_Create(), TestUserService_Delete(), TestUserService_GetByUsername() (+3 more)

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
Cohesion: 0.22
Nodes (12): authRepository, useCurrentUser(), useLogin(), useLogout(), useAuthStore, AppLayout(), Header(), Sidebar() (+4 more)

### Community 59 - "TikMan - ZTE OLT Provisioning System"
Cohesion: 0.09
Nodes (22): API Endpoints, Authentication, Code Formatting, Default Credentials, Development, Development Setup, Features, License (+14 more)

### Community 60 - "UserService"
Cohesion: 0.06
Nodes (22): CreateUserRequest, UpdateUserRequest, Data, MemoryStore, TestHealthEndpoint(), TestRouterSetup(), NewMemoryStore(), Store (+14 more)

### Community 61 - "🔧 Troubleshooting: Duplicate ONT Serial Numbers"
Cohesion: 0.09
Nodes (21): 1. Check Worker Logs, 2. Check Dashboard, 3. Verify Metrics Values, Fix Implementation, How It Happens, Method A: Via Web Dashboard (EASIEST), Method B: From Hardware Labels (MOST RELIABLE), Method C: Network Discovery via SNMP (+13 more)

### Community 62 - "dto.go"
Cohesion: 0.19
Nodes (13): CreateOLTRequest, CreateSiteRequest, ErrorResponse, LoginRequest, LoginResponse, OLTResponse, TestConnectionRequest, TestConnectionResponse (+5 more)

### Community 63 - "File Structure"
Cohesion: 0.11
Nodes (18): Execution Notes, File Structure, Global Constraints, OLT Validation and Connection Test Implementation Plan, Self-Review Checklist, Task 10: Update Dependency Injection in Main, Task 11: Update OLTService Unit Tests, Task 12: Integration Testing and Documentation (+10 more)

### Community 64 - "Code Quality Refactoring Report - TikMan"
Cohesion: 0.11
Nodes (17): 1. Updated CLAUDE.md ✅, 2. Created test_helpers.go ✅, 3. Refactored Test Files ✅, 4. Fixed Handler Nil Checks ✅, Architecture Quality, Changes Made, Code Quality Metrics, Code Quality Refactoring Report - TikMan (+9 more)

### Community 65 - "Olts.tsx"
Cohesion: 0.18
Nodes (11): oltRepository, useCreateOlt(), useDeleteOlt(), useOlts(), useUpdateOlt(), DarkCard(), DarkCardProps, PageHeader() (+3 more)

### Community 66 - "Global Constraints"
Cohesion: 0.12
Nodes (15): Global Constraints, Task 10: Site Service & Handlers, Task 11: OLT Service & Handlers, Task 12: Server Setup with Routes, Task 13: Docker Compose Setup, Task 1: Project Scaffolding & Dependencies, Task 2: Database Connection & Logger Setup, Task 3: Database Models (+7 more)

### Community 67 - "NewSiteService"
Cohesion: 0.45
Nodes (12): NewOLTService(), TestOLTService_Create(), TestOLTService_Create_DuplicateIP(), TestOLTService_Create_InvalidSiteID(), TestOLTService_DecryptPassword(), TestOLTService_Delete(), TestOLTService_GetByID(), TestOLTService_List() (+4 more)

### Community 68 - "SiteService"
Cohesion: 0.14
Nodes (6): SiteHandler, SiteResponse, ToSiteResponse(), NewSiteHandler(), Site, SiteService

### Community 69 - "Source of Truth (SOT) - TikMan"
Cohesion: 0.05
Nodes (37): API Contract, Architecture Principles, Backend Modules, Change Log, CI/CD Pipeline, `cmd/api/`, Database Schema, DD-001: No GORM Preload Usage (+29 more)

### Community 70 - "Manual Testing - OLT Validation"
Cohesion: 0.17
Nodes (11): Expected Response Format, Known Test Behavior, Linting, Manual Testing - OLT Validation, Run Specific Test, Running Tests, Test Scenarios, Test with Mock OLT (+3 more)

### Community 71 - "SetupOLTHandlerTest"
Cohesion: 0.33
Nodes (10): TestOLTHandler_Create(), TestOLTHandler_Delete(), TestOLTHandler_GetByID(), TestOLTHandler_List(), TestOLTHandler_Update(), CreateTestOLT(), SetupAuthHandlerTest(), SetupOLTHandlerTest() (+2 more)

### Community 72 - "CLAUDE.md"
Cohesion: 0.05
Nodes (38): Adding a New API Endpoint, Adding a New Model, AI-Generated Code Standards, AI Slop Review Checklist, Anti-Patterns to Avoid, Architecture, Backend (Go), Backend Structure (Go) (+30 more)

### Community 73 - "setupRBACRouter"
Cohesion: 0.31
Nodes (8): RequireRole(), setupRBACRouter(), TestRequireRole_Allowed(), TestRequireRole_Forbidden(), TestRequireRole_NoRole(), TestRequireRole_SingleRoleMatch(), TestRequireRole_TechnicianAllowed(), github.com/gin-gonic/gin.Engine

### Community 74 - "TikMan OLT Provisioning - Frontend Application Design Specification"
Cohesion: 0.20
Nodes (9): 11. File Naming Conventions, 13. Performance Targets, 14. Browser Support, 15. Accessibility (WCAG 2.1 Level AA), 17. Success Criteria, 2. Clean Architecture Principles, 3. Project Structure, Architecture Layers (+1 more)

### Community 75 - "4. API Endpoints"
Cohesion: 0.20
Nodes (10): 4. API Endpoints, Audit Logs, Authentication, Dashboard & Monitoring, Health Check, OLTs, ONTs, Service & Line Profiles (+2 more)

### Community 76 - "Monitoring Module Phase 2: Metrics Collection Implementation Plan"
Cohesion: 0.22
Nodes (8): Final Verification, Global Constraints, Monitoring Module Phase 2: Metrics Collection Implementation Plan, Task 1: SNMP Metrics Query Implementation, Task 2: Metrics Storage Service, Task 3: Metrics Collection Worker, Task 4: Metrics API Endpoints, Task 5: Frontend Metrics Display

### Community 77 - "NewOLTValidatorService"
Cohesion: 0.50
Nodes (7): NewOLTValidatorService(), setupValidatorTestDB(), TestNewOLTValidatorService(), TestValidateCreate_LocalhostSSH(), TestValidateIPNotDuplicate_Duplicate(), TestValidateIPNotDuplicate_NoDuplicate(), TestValidationResult_StructureExists()

### Community 78 - "scripts"
Cohesion: 0.25
Nodes (8): scripts, build, dev, format, lint, preview, test, test:ui

### Community 79 - "SetupSiteHandlerTest"
Cohesion: 0.48
Nodes (6): TestSiteHandler_Create(), TestSiteHandler_Delete(), TestSiteHandler_GetByID(), TestSiteHandler_List(), TestSiteHandler_Update(), SetupSiteHandlerTest()

### Community 80 - "3. Database Schema"
Cohesion: 0.25
Nodes (8): 3. Database Schema, audit_logs, line_profiles, olts, onts, service_profiles, sites, users

### Community 81 - "TikMan Frontend"
Cohesion: 0.25
Nodes (7): Architecture, Development, Docker, Environment Variables, Project Structure, Tech Stack, TikMan Frontend

### Community 82 - "time.Time"
Cohesion: 0.12
Nodes (10): MetricsHandler, ONTMetricsResponse, ToONTMetricsResponse(), NewMetricsHandler(), formatPowerValue(), MetricsService, ONTMetricsRow, gorm.io/datatypes.JSON (+2 more)

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

### Community 87 - "setupSiteTestDB"
Cohesion: 0.52
Nodes (6): setupSiteTestDB(), TestSiteService_Create(), TestSiteService_Delete(), TestSiteService_GetByID(), TestSiteService_List(), TestSiteService_Update()

### Community 88 - "New"
Cohesion: 0.40
Nodes (4): New(), TestNew_InvalidLogLevel(), TestNew_TimestampFormatUTC(), TestNew_ValidLogLevels()

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

### Community 94 - "package.json"
Cohesion: 0.33
Nodes (5): name, overrides, path-to-regexp, type, version

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

## Knowledge Gaps
- **613 isolated node(s):** `FIX_DUPLICATE_SERIALS.sh script`, `github.com/tikman/olt-provisioning`, `LoginRequest`, `ErrorResponse`, `CreateSiteRequest` (+608 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **17 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `MonitoringWorker` connect `snmp.go` to `time.Time`, `github.com/google/uuid.UUID`, `time.Duration`, `gorm.io/gorm.DB`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **Why does `CreateTestOLT()` connect `SetupOLTHandlerTest` to `testing.T`, `github.com/google/uuid.UUID`, `gorm.io/gorm.DB`, `UserService`, `dto.go`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **Why does `ToOLTResponse()` connect `github.com/gin-gonic/gin.Context` to `dto.go`, `gorm.io/gorm.DB`?**
  _High betweenness centrality (0.005) - this node is a cross-community bridge._
- **What connects `FIX_DUPLICATE_SERIALS.sh script`, `github.com/tikman/olt-provisioning`, `LoginRequest` to the rest of the system?**
  _613 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Olt` be split into smaller, more focused modules?**
  _Cohesion score 0.1330049261083744 - nodes in this community are weakly interconnected._
- **Should `hooks.test.ts` be split into smaller, more focused modules?**
  _Cohesion score 0.11341463414634147 - nodes in this community are weakly interconnected._
- **Should `time.Duration` be split into smaller, more focused modules?**
  _Cohesion score 0.09475806451612903 - nodes in this community are weakly interconnected._