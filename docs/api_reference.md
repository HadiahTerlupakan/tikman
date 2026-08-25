# API Reference

## Base URL
```
http://localhost:8080/api/v1
```

## Authentication
All protected endpoints require authentication via HTTP-only cookie session. Login at `/login` returns JWT cookie automatically.

---

## Config Templates

### List All Templates
```
GET /api/v1/config-templates
Authorization: Bearer
```
**Response:** `200 OK`
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Default ZTE Profile",
      "description": "...",
      "vendor": "ZTE",
      "config_fields": {},
      "is_default": true,
      "created_at": "2026-08-24T...",
      "updated_at": "2026-08-24T..."
    }
  ],
  "total": 3
}
```

### Get Template by ID
```
GET /api/v1/config-templates/:id
Authorization: Bearer
```
**Response:** `200 OK` or `404 NOT_FOUND`

### Create Template
```
POST /api/v1/config-templates
Authorization: Bearer
Content-Type: application/json
```
**Request Body:**
```json
{
  "name": "Custom Profile",
  "description": "My custom template",
  "vendor": "ZTE|HSGQ",
  "config_fields": {"vlan": "100"},
  "is_default": false
}
```
**Response:** `201 Created` with full template object

### Update Template
```
PUT /api/v1/config-templates/:id
Authorization: Bearer (Admin/Tech role)
```
**Response:** `200 OK`

### Delete Template
```
DELETE /api/v1/config-templates/:id
Authorization: Bearer (Admin role)
```
**Response:** `200 OK` or `409 CONFLICT` if referenced by jobs

---

## Single ONT Provisioning

### Start Provisioning
```
POST /api/v1/onts/:id/provision
Authorization: Bearer
Content-Type: application/json
Role: Admin/Technician
```
**Request Body:**
```json
{
  "template_id": "uuid-or-null",
  "manual_config": {"vlan": "100"},
  "confirm": true
}
```
**Response:** `200 OK`
```json
{
  "job_id": "uuid",
  "status": "pending|running|success|failed",
  "message": "provisioning completed"
}
```

### Get Job Status
```
GET /api/v1/provision-jobs/:id
Authorization: Bearer
```
**Response:** `200 OK` with job details including errorMessage if failed

### List Jobs for ONT
```
GET /api/v1/onts/:id/provision-jobs?limit=20&offset=0
Authorization: Bearer
```
**Response:** Paginated list of provisioning jobs

---

## Batch Provisioning

### Start Batch Provision
```
POST /api/v1/batch-provision
Authorization: Bearer
Content-Type: application/json
Role: Admin/Technician
```
**Request Body:**
```json
{
  "template_id": "uuid-required",
  "ont_ids": ["uuid1", "uuid2", ...],
  "manual_config": {},
  "confirm": true
}
```
**Response:** `200 OK`
```json
{
  "job_id": "uuid",
  "status": "running|success|failed|partial_rollback",
  "succeeded": ["uuid"],
  "failed": ["uuid"],
  "rolled_back": ["uuid"],
  "details": {
    "uuid1": {"status": "success"},
    "uuid2": {"status": "failed", "error": "SNMP timeout"}
  }
}
```

### Get Batch Job Status
```
GET /api/v1/batch-jobs/:id
Authorization: Bearer
```
**Response:** `200 OK` with full batch result

---

## Error Response Format
```json
{
  "code": "INVALID_ID|NOT_FOUND|CONFLICT|VALIDATION_ERROR|NOT_CONFIRMED|INTERNAL_ERROR",
  "error": "Human-readable error message",
  "details": {}  // optional additional info
}
```

## Validation Guards
- UUID format validated for all `:id` path params
- `confirm=true` required for all provisioning operations
- Empty ONT list rejected for batch operations
- Template name must be unique and 3-100 characters
