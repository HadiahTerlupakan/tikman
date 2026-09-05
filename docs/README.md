# Documentation

Project documentation and guides.

## Structure

### `/archive`
Historical documentation, fix summaries, and implementation notes:
- `ONT_MONITORING_QUICK_START.md` - Quick start guide for ONT monitoring
- `INSTRUCTIONS_TO_FIX_YOUR_OLT.md` - OLT troubleshooting guide
- `TROUBLESHOOTING_DUPLICATE_SERIALS.md` - Duplicate serial troubleshooting
- `SOT.md` - Source of Truth documentation

### Root (`/docs`)
Active documentation:
- `operator_guide.md` - Day-to-day operations: provisioning, VPN sites, OLT map, deploy
- `api_reference.md` - Config templates and single-ONT provisioning endpoints
- `SECURITY.md` - Security guidelines and best practices

Dated records, kept as history rather than as current reference:
- `SECURITY_AUDIT.md` - Security audit, 2026-08-15
- `MONITORING_MODULE_DESIGN.md` - Monitoring module design draft, 2026-08-15;
  its polling design was superseded by the job-queue tiers described in `CLAUDE.md`

### `/superpowers`
Dated design specs and implementation plans. Snapshots of what was decided at
the time — not maintained against the code.

## Usage

Refer to the main [README.md](../README.md) for project overview.

For setup instructions, see backend and frontend specific documentation in their respective directories.
