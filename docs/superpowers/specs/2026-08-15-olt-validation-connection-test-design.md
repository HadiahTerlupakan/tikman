# OLT Validation and Connection Test Design

**Date:** 2026-08-15  
**Feature:** Add validation and connection testing to Create OLT  
**Approach:** Separate Validator Service with Inline Connection Testing

---

## Overview

Add comprehensive validation and connection testing to the OLT creation flow to ensure that OLT devices are reachable and credentials are valid before saving to the database. This prevents invalid OLT configurations from being stored.

---

## Requirements

1. **Validation must happen before database save** - Invalid OLT data must not be persisted
2. **Strict validation mode** - All tests must pass (Ping + SSH/Telnet + SNMP)
3. **Fast timeout** - Total validation time ~5 seconds (Ping: 2s, SSH/Telnet: 2s, SNMP: 1s)
4. **Progressive error messaging** - Show which tests passed and where validation failed
5. **Status management** - Set OLT status to "online" only when all tests pass

---

## Architecture

### Component Overview

```
┌─────────────────────┐
│  OLTHandler         │
│  (HTTP Layer)       │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  OLTService         │
│  (Business Logic)   │
└──────────┬──────────┘
           │
           ├─────────────────┐
           │                 │
           ▼                 ▼
┌─────────────────────┐  ┌──────────────────┐
│ OLTValidatorService │  │  Database (GORM) │
│ (Validation Logic)  │  └──────────────────┘
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  connectivity       │
│  (Network Tests)    │
└─────────────────────┘
```

### New Package: `internal/connectivity`

Low-level network connection testing functions:

**Functions:**
- `PingTest(ipAddress string, timeout time.Duration) error`
  - ICMP ping test using `github.com/go-ping/ping`
  - Returns error if host unreachable or timeout

- `SSHTest(ipAddress string, port int, username, password string, timeout time.Duration) error`
  - SSH connection and authentication test using `golang.org/x/crypto/ssh`
  - Returns error if connection fails or auth fails

- `TelnetTest(ipAddress string, port int, username, password string, timeout time.Duration) error`
  - Telnet connection and basic auth test using `net.DialTimeout`
  - Returns error if connection fails or timeout

- `SNMPTest(ipAddress string, port int, community string, timeout time.Duration) error`
  - SNMP connectivity test using `github.com/gosnmp/gosnmp`
  - Returns error if agent unreachable or invalid community

### New Service: `internal/services/olt_validator_service.go`

Validation orchestration service:

**Struct:**
```go
type OLTValidatorService struct {
    db *gorm.DB
}

type ValidationResult struct {
    Success      bool
    PassedTests  []string  // e.g., ["Ping", "SSH"]
    FailedTest   string    // e.g., "SNMP"
    FailedReason string    // e.g., "timeout after 1s"
}
```

**Methods:**

- `ValidateIPNotDuplicate(ipAddress string) error`
  - Check if IP address already exists in database
  - Returns error if duplicate found

- `ValidateCreate(ipAddress, username, password string, sshPort, telnetPort, snmpPort int, snmpCommunity string, preferredProtocol models.OLTProtocol) (*ValidationResult, error)`
  - Orchestrates all connection tests in sequence
  - Stops at first failure (progressive validation)
  - Returns ValidationResult with passed/failed test details

### Modified Service: `internal/services/olt_service.go`

**Updated `Create()` method flow:**

1. Validate site exists (existing)
2. Call `validatorService.ValidateIPNotDuplicate(ipAddress)`
3. Call `validatorService.ValidateCreate(...)` for connection tests
4. If validation fails → return progressive error, do not save
5. If validation passes → encrypt password → save to DB with status "online"
6. Create audit log
7. Return OLT response

---

## Data Flow

### Success Flow

```
POST /api/olts (Create OLT Request)
    ↓
OLTHandler.Create() - Validate JSON
    ↓
OLTService.Create()
    ↓
Check site exists ✓
    ↓
OLTValidatorService.ValidateIPNotDuplicate() ✓
    ↓
OLTValidatorService.ValidateCreate()
    ├─ connectivity.PingTest() [2s] ✓
    ├─ connectivity.SSHTest() or TelnetTest() [2s] ✓
    └─ connectivity.SNMPTest() [1s] ✓
    ↓
All tests passed ✓
    ↓
Encrypt password
    ↓
Save to DB (status = "online")
    ↓
Create audit log
    ↓
Return 201 Created with OLTResponse
```

### Failure Flow (Example: SNMP timeout)

```
POST /api/olts (Create OLT Request)
    ↓
OLTHandler.Create() - Validate JSON
    ↓
OLTService.Create()
    ↓
Check site exists ✓
    ↓
OLTValidatorService.ValidateIPNotDuplicate() ✓
    ↓
OLTValidatorService.ValidateCreate()
    ├─ connectivity.PingTest() [2s] ✓
    ├─ connectivity.SSHTest() [2s] ✓
    └─ connectivity.SNMPTest() [1s] ✗ TIMEOUT
    ↓
Validation failed
    ↓
Return ValidationResult{
    Success: false,
    PassedTests: ["Ping", "SSH"],
    FailedTest: "SNMP",
    FailedReason: "timeout after 1s"
}
    ↓
OLTService returns error (do not save to DB)
    ↓
OLTHandler returns 400 Bad Request
```

---

## Error Handling

### Progressive Error Response Format

When validation fails, return detailed error with progressive results:

**HTTP Status:** 400 Bad Request

**Response Body:**
```json
{
  "error": "OLT validation failed",
  "code": "VALIDATION_FAILED",
  "details": {
    "passed_tests": ["Ping", "SSH"],
    "failed_test": "SNMP",
    "failed_reason": "timeout after 1s"
  }
}
```

### Error Scenarios

1. **Duplicate IP:**
   - Code: `DUPLICATE_IP`
   - Message: "IP address already exists"
   - Details: IP address value

2. **Ping failure:**
   - PassedTests: []
   - FailedTest: "Ping"
   - FailedReason: "host unreachable" or "timeout after 2s"

3. **SSH/Telnet failure:**
   - PassedTests: ["Ping"]
   - FailedTest: "SSH" or "Telnet"
   - FailedReason: "authentication failed", "connection refused", or "timeout after 2s"

4. **SNMP failure:**
   - PassedTests: ["Ping", "SSH"]
   - FailedTest: "SNMP"
   - FailedReason: "timeout after 1s" or "invalid community string"

---

## Testing Strategy

### Unit Tests

**`internal/connectivity` package:**
- Test each function with mock network scenarios
- Mock success, timeout, unreachable, auth failed cases
- Verify timeout enforcement
- Verify error messages

**`internal/services/olt_validator_service.go`:**
- Mock connectivity package
- Test IP duplicate detection
- Test validation stops at first failure
- Test progressive error message generation
- Test all tests passing scenario

**`internal/services/olt_service.go`:**
- Mock OLTValidatorService
- Test Create with validation pass → saves with status "online"
- Test Create with validation fail → returns error, no save
- Test Create with duplicate IP → returns error
- Test Create with invalid site → returns error

### Integration Tests

- Test with mock OLT simulator (if available)
- Test with invalid credentials to verify auth failure detection
- Test with unreachable IP to verify timeout behavior
- Test end-to-end flow from HTTP request to database

### Manual Testing Checklist

- [ ] Create OLT with valid credentials and reachable IP → Success
- [ ] Create OLT with wrong password → Fails at SSH/Telnet test
- [ ] Create OLT with unreachable IP → Fails at Ping test
- [ ] Create OLT with invalid SNMP community → Fails at SNMP test
- [ ] Create OLT with duplicate IP → Fails with duplicate error
- [ ] Verify timeout enforcement (network delay simulation)
- [ ] Verify progressive error messages are accurate

---

## Dependencies

### Go Libraries

1. **Ping Test:** `github.com/go-ping/ping` (v1.1.0+)
   - Reliable ICMP ping library
   - Cross-platform support
   - Timeout and packet loss detection

2. **SSH Test:** `golang.org/x/crypto/ssh` (already in go.mod)
   - Standard Go SSH client
   - Password authentication support

3. **Telnet Test:** Standard library `net` package
   - No external dependency
   - Use `net.DialTimeout()`

4. **SNMP Test:** `github.com/gosnmp/gosnmp` (v1.37.0+)
   - Pure Go SNMP library
   - SNMPv2c support
   - Timeout support

### Installation

```bash
cd backend
go get github.com/go-ping/ping@latest
go get github.com/gosnmp/gosnmp@latest
```

### Notes

- ICMP ping may require root/admin privileges on some systems
- Alternative: TCP connection test to SSH/Telnet port as ping substitute if permission issues arise
- SNMPv2c is assumed (most common for OLT devices)

---

## Implementation Files

### New Files

1. `backend/internal/connectivity/ping.go` - Ping test implementation
2. `backend/internal/connectivity/ssh.go` - SSH test implementation
3. `backend/internal/connectivity/telnet.go` - Telnet test implementation
4. `backend/internal/connectivity/snmp.go` - SNMP test implementation
5. `backend/internal/connectivity/connectivity_test.go` - Unit tests for connectivity package
6. `backend/internal/services/olt_validator_service.go` - Validator service
7. `backend/internal/services/olt_validator_service_test.go` - Validator service tests

### Modified Files

1. `backend/internal/services/olt_service.go` - Add validation calls in Create()
2. `backend/internal/services/olt_service_test.go` - Update tests to include validation
3. `backend/internal/api/olt_handler.go` - Update error handling for validation errors
4. `backend/cmd/api/main.go` - Initialize OLTValidatorService and inject into OLTService

---

## Migration Plan

### Phase 1: Connectivity Package
- Implement ping, SSH, telnet, SNMP test functions
- Write unit tests with mocks
- Verify timeout behavior

### Phase 2: Validator Service
- Implement OLTValidatorService
- Implement IP duplicate check
- Implement ValidateCreate orchestration
- Write unit tests

### Phase 3: Integration
- Update OLTService.Create() to use validator
- Update OLTHandler error handling
- Update dependency injection in main.go
- Update OLTService tests

### Phase 4: Testing & Documentation
- Integration tests
- Manual testing with real/mock OLT
- Update API documentation
- Update CLAUDE.md if needed

---

## Security Considerations

1. **Password exposure in logs:**
   - Never log plaintext passwords in connectivity tests
   - Log only connection success/failure, not credentials

2. **Timeout enforcement:**
   - All network operations must have timeouts
   - Prevent hanging connections from blocking requests

3. **Error message sanitization:**
   - Progressive errors are detailed but don't expose sensitive internal info
   - Balance debugging information with security

4. **SNMP community strings:**
   - Treat SNMP community as sensitive data
   - Don't log community strings

---

## Success Criteria

✅ All connection tests must pass before OLT is saved to database  
✅ Validation completes within 5 seconds (Ping 2s + SSH/Telnet 2s + SNMP 1s)  
✅ Progressive error messages show which tests passed and where failure occurred  
✅ OLT status is set to "online" only when all tests pass  
✅ Duplicate IP addresses are rejected before connection tests  
✅ All unit tests pass  
✅ Integration tests verify end-to-end flow  
✅ Manual testing confirms validation behavior matches requirements  

---

## Future Enhancements (Out of Scope)

- Async validation with background jobs (if blocking validation becomes UX issue)
- Retry logic for transient network failures
- Connection test results caching
- Validation on OLT Update (reuse validator service)
- Health check endpoint that runs validation on existing OLTs
- Metrics/monitoring for validation success/failure rates
