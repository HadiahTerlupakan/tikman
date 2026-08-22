# Scripts

Utility scripts for maintenance, fixes, and testing.

## Structure

### `/fixes`
Database fixes and maintenance scripts:
- `FIX_DUPLICATE_SERIALS.sh` - Fix duplicate ONT serial numbers
- `RUN_THIS_UPDATE.sql` - Database schema updates
- `SQL_BATCH_UPDATE_OLTS.sql` - Batch update OLT configurations
- `check_and_fix_olts.sql` - Check and fix OLT data

### `/testing`
Test scripts and utilities:
- `decode_ifindex.go` - Test ifIndex decoding
- `test_*.go` - SNMP testing scripts
- `setup-ont-monitoring.sh` - ONT monitoring setup
- `test-topology.sh` - Topology testing

### `/maintenance`
Reserved for future maintenance scripts.

### Git hooks
- `install-hooks.sh` - Installs the repo's hooks into `.git/hooks/`. Run once per clone.
- `pre-commit` - Source for the hook; runs prettier over staged `frontend/src` files.

## Usage

### Installing Git Hooks

```bash
./scripts/install-hooks.sh
```

Formats staged frontend files on commit so the CI formatting gate cannot fail on
avoidable drift. `SKIP_FORMAT_HOOK=1 git commit` bypasses it.

### Running Fix Scripts

**Before running any fix script:**
1. Backup your database
2. Review the script content
3. Test in development environment first

**Example:**
```bash
# Backup database first
docker exec tikman-postgres pg_dump -U tikman tikman > backup.sql

# Run fix script
cd scripts/fixes
./FIX_DUPLICATE_SERIALS.sh
```

### Running Test Scripts

**SNMP Tests:**
```bash
cd scripts/testing
go run test_snmp_final.go
```

**Setup Scripts:**
```bash
cd scripts/testing
chmod +x setup-ont-monitoring.sh
./setup-ont-monitoring.sh
```

## Safety Notes

- Always backup before running fix scripts
- Test scripts are for development/debugging only
- Review SQL scripts before execution
- Check logs after running maintenance scripts
