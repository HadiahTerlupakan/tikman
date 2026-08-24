package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// migrationFile is one parsed SQL migration with its ordering prefix.
type migrationFile struct {
	version string // numeric prefix, e.g. "02" from 02_create_timeseries_tables.sql
	path    string
}

// RunSQLMigrations applies every .sql file in dir (sorted by filename) to db,
// skipping ones already recorded in the schema_migrations table. PostgreSQL
// migrations only — SQLite test databases are skipped because they use
// AutoMigrate and cannot run TimescaleDB SQL.
func RunSQLMigrations(db *gorm.DB, dir string) error {
	dialect := db.Name()
	if dialect != "postgres" {
		// SQLite (tests) builds its schema via AutoMigrate; the SQL files
		// contain TimescaleDB/PostgreSQL-only syntax.
		return nil
	}

	if err := ensureMigrationTable(db); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := loadMigrationFiles(dir)
	if err != nil {
		return fmt.Errorf("load migrations from %s: %w", dir, err)
	}

	for _, file := range files {
		applied, err := isApplied(db, file.version)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", file.version, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := os.ReadFile(file.path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file.version, err)
		}

		// Some TimescaleDB statements (materialized views) cannot run inside
		// a transaction. Execute the migration as a script, then record it.
		if err := executeMigration(db, string(sqlBytes), file.version); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationTable(db *gorm.DB) error {
	return db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(20) PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error
}

func loadMigrationFiles(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []migrationFile
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".down.sql") {
			continue // rollback scripts are not part of forward migrations
		}
		// Numeric prefix before the first underscore is the version. Both
		// `02_name.sql` and `000003_name.up.sql` are supported.
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 || !isNumeric(parts[0]) {
			continue
		}
		files = append(files, migrationFile{version: parts[0], path: filepath.Join(dir, name)})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

func isApplied(db *gorm.DB, version string) (bool, error) {
	var count int64
	err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count).Error
	return count > 0, err
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
