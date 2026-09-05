# Manual Testing - OLT Validation

## Testing OLT Creation with Validation

The OLT creation now includes validation that requires:
1. Ping test (2s timeout)
2. SSH/Telnet connection test (2s timeout)
3. SNMP connectivity test (1s timeout)

### Test Scenarios

**Success Case:**
- Use a reachable OLT device with correct credentials
- All tests should pass
- OLT saved with status "online"

**Failure Cases:**
- Unreachable IP → Fails at Ping
- Wrong credentials → Fails at SSH/Telnet
- Wrong SNMP community → Fails at SNMP
- Duplicate IP → Fails before connection tests

### Test with Mock OLT
For testing without real hardware, you can:
1. Set up SSH server on localhost
2. Set up SNMP agent (snmpd)
3. Create OLT pointing to 127.0.0.1

### Expected Response Format
Validation failure returns `400 Bad Request`. A failure at Ping or SSH/Telnet:
```json
{
  "error": "OLT validation failed",
  "code": "VALIDATION_FAILED",
  "details": "OLT validation failed - Passed: [Ping, SSH], Failed: SNMP (timeout after 1s)"
}
```
A failure at SNMP carries its own code, so the operator sees which stage broke:
```json
{
  "error": "SNMP connection test failed",
  "code": "SNMP_TEST_FAILED",
  "details": "SNMP connection test failed: ..."
}
```

## Running Tests

### Unit Tests
```bash
cd backend
go test ./... -v
```

### With Coverage
```bash
cd backend
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Run Specific Test
```bash
cd backend
go test -v -run TestValidateCreate_LocalhostSSH ./internal/services/
```

## Linting

```bash
cd backend
golangci-lint run ./...
```

## Catatan Validasi

Handler tests run the real validator, so creating an OLT at an unreachable IP
fails — and that failure is what they assert. `TestOLTHandler_Create` expects
`400` with code `SNMP_TEST_FAILED`, which is why the suite is green rather than
in spite of it. It does assume `192.168.1.1` is unreachable from the machine
running the tests; on a LAN where that address answers SNMP, the assertion
flips.

Service-level tests exercise the same validator directly and document the
failures per stage. `TestValidateCreate_LocalhostSSH` depends on whether SSH is
listening on `localhost:22`.

Every test in the suite is expected to pass. A red suite is a defect, never
"expected behaviour" — see the pre-commit gate in `CLAUDE.md`.
