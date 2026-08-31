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
	// dollarTag holds the opening tag of a dollar-quoted block, empty outside
	// one. Postgres closes such a block only on its own tag, so $fix$ is not
	// ended by a bare $$ and everything between is literal — semicolons
	// included. Splitting inside one hands the database a fragment ending
	// mid-block, which fails as "unterminated dollar-quoted string".
	dollarTag := ""

	runes := []rune(sqlText)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if dollarTag != "" {
			if tag, width := readDollarTag(runes, i); tag == dollarTag {
				current.WriteString(tag)
				i += width - 1
				dollarTag = ""
				continue
			}
			current.WriteRune(r)
			continue
		}

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
		case '$':
			if !inSingle && !inDouble {
				if tag, width := readDollarTag(runes, i); tag != "" {
					current.WriteString(tag)
					i += width - 1
					dollarTag = tag
					continue
				}
			}
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

// readDollarTag reads a dollar-quote delimiter at position i — "$$" or "$tag$"
// — returning the delimiter and how many runes it spans. An empty tag means
// there is none there, which is how a dollar inside a string or a price stays
// an ordinary character.
func readDollarTag(runes []rune, i int) (string, int) {
	if runes[i] != '$' {
		return "", 0
	}
	for j := i + 1; j < len(runes); j++ {
		if runes[j] == '$' {
			return string(runes[i : j+1]), j + 1 - i
		}
		if !isTagRune(runes[j], j == i+1) {
			return "", 0
		}
	}
	return "", 0
}

// isTagRune reports whether a rune may appear in a dollar-quote tag. Postgres
// spells the tag as an identifier, so it cannot start with a digit.
func isTagRune(r rune, first bool) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		return true
	case r >= '0' && r <= '9':
		return !first
	default:
		return false
	}
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
