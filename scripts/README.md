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
- `test-topology.sh` - Triggers topology discovery on the first OLT the API
  lists, and prints the first ONT it returns.

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

`test-topology.sh` calls the API on `localhost:8080` and starts a real discovery
against a real chassis, so run it only when that is what you want.

```bash
./scripts/testing/test-topology.sh
```

## Safety Notes

- Always backup before running fix scripts
- Test scripts are for development/debugging only
- Review SQL scripts before execution
- Check logs after running maintenance scripts
