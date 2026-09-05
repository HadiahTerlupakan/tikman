# API Reference

Covers config templates and single-ONT provisioning. Other surfaces (sites,
OLTs, ONTs, users, CS inbox, WireGuard, health) are not documented here yet.

## Base URL
```
http://localhost:8080/api/v1
```

## Authentication

Every endpoint below requires an authenticated session. `POST /api/v1/auth/login`
sets an HTTP-only `session_token` cookie backed by Redis; send that cookie on
subsequent requests. There is no bearer token — `Authorization` headers are
ignored, and a request without the cookie gets `401 UNAUTHORIZED`.

Endpoints that also need a role say so. The rest accept any authenticated user.

---

## Config Templates

### List All Templates
```
GET /api/v1/config-templates
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
```
**Response:** `200 OK` or `404 NOT_FOUND`

### Create Template
```
POST /api/v1/config-templates
Content-Type: application/json
Role: Admin/Technician
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
Role: Admin/Technician
```
**Response:** `200 OK`

### Delete Template
```
DELETE /api/v1/config-templates/:id
Role: Admin
```
**Response:** `200 OK` or `409 CONFLICT` if referenced by jobs

---

## Single ONT Provisioning

### Start Provisioning
```
POST /api/v1/onts/:id/provision
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
  "status": "pending|running|success|failed|rolled_back",
  "message": "provisioning completed"
}
```

### Get Job Status
```
GET /api/v1/provision-jobs/:id
```
**Response:** `200 OK` with job details including errorMessage if failed

### List Jobs for ONT
```
GET /api/v1/onts/:id/provision-jobs?limit=20&offset=0
```
**Response:** Paginated list of provisioning jobs

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
- Template name must be unique and 3-100 characters
