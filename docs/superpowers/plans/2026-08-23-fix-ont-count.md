# Fix ont_count Display Bug — Implementation Plan

**Goal:** Fix `ont_count` in OLT response from 0 to actual count.

**Root cause:** `ToOLTResponse` hardcodes `ONTCount: 0`; no query to get count exists.

**Strategy:** Add `CountONTsByOLT` method to `OntService`, call it in OLT handler List endpoint, pass count to DTO.

## Tasks

### Task 1: Add CountONTsByOLT method to OntService

**File:** `internal/services/ont_service.go`

Add after line ~325 (ListONTSummariesForOLT):

```go
// CountONTsByOLT returns how many ONTs belong to a single OLT.
func (s *ONTService) CountONTsByOLT(oltID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&count).Error
	return count, err
}
```

### Task 2: Update ToOLTResponse to accept count

**File:** `internal/api/olt_dto.go`

Change signature from:
```go
func ToOLTResponse(siteName string, olt *models.OLT) OLTResponse
```

to:
```go
func ToOLTResponse(siteName string, oltCount int64, olt *models.OLT) OLTResponse
```

Update body to use `oltCount` parameter instead of hardcoded 0.

### Task 3: Update all callers of ToOLTResponse

Files to update:
- `internal/api/olt_handler_crud.go`: Line with `ToOLTResponse(siteName, &olt)` → add count
- `internal/api/olt_handler_crud_update.go`: Same
- `internal/api/olt_handler_crud_create.go`: Same

Pattern for each:
```go
ontCount, _ := h.ontService.CountONTsByOLT(olt.ID)
responses[i] = ToOLTResponse(siteName, ontCount, &olt)
```

### Task 4: Test and verify

Run:
```bash
cd backend
gofmt -s -l .
go vet ./...
go build ./...
go test ./... -race
curl -b /tmp/tikman_cookies.txt 'http://localhost:8080/api/v1/olts' | grep ont_count
```

Expected: Cariu has ont_count=198, RENGAS has ont_count=246.

## Notes

- Minimal invasive change
- All existing tests should still pass
- Error handling for count failure: ignore count if error (return 0 as fallback)
