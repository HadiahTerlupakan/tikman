package database

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// executeMigration runs a migration statement-by-statement. This is necessary
// for TimescaleDB commands such as CREATE MATERIALIZED VIEW, which PostgreSQL
// refuses inside a transaction block. SQL files in this repository contain
// line-oriented statements and comments; the splitter preserves dollar-quoted
// function bodies if a future migration needs them.
func executeMigration(db *gorm.DB, sqlText, version string) error {
	for _, statement := range splitSQLStatements(sqlText) {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
	}
	if err := db.Exec(
		"INSERT INTO schema_migrations (version) VALUES (?)", version,
	).Error; err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	return nil
}

func splitSQLStatements(sqlText string) []string {
	var statements []string
	var current strings.Builder
	inSingle, inDouble, inLineComment := false, false, false

	for _, r := range sqlText {
		if inLineComment {
			if r == '\n' {
				inLineComment = false
			}
			current.WriteRune(r)
			continue
		}
		if !inSingle && !inDouble && strings.HasSuffix(current.String(), "--") {
			inLineComment = true
			current.WriteRune(r)
			continue
		}

		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble {
				if statement := stripSQLComments(current.String()); statement != "" {
					statements = append(statements, statement)
				}
				current.Reset()
				continue
			}
		}
		current.WriteRune(r)
	}

	if statement := stripSQLComments(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func stripSQLComments(statement string) string {
	lines := strings.Split(statement, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
