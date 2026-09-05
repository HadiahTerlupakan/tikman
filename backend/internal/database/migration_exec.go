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

// sqlSplitter walks a migration file one rune at a time, keeping track of the
// contexts in which a semicolon does not end a statement.
type sqlSplitter struct {
	statements []string
	current    strings.Builder

	inSingle      bool
	inDouble      bool
	inLineComment bool
	// dollarTag holds the opening tag of a dollar-quoted block, empty outside
	// one. Postgres closes such a block only on its own tag, so $fix$ is not
	// ended by a bare $$ and everything between is literal — semicolons
	// included. Splitting inside one hands the database a fragment ending
	// mid-block, which fails as "unterminated dollar-quoted string".
	dollarTag string
}

func splitSQLStatements(sqlText string) []string {
	splitter := &sqlSplitter{}

	runes := []rune(sqlText)
	for i := 0; i < len(runes); i++ {
		i += splitter.step(runes, i)
	}
	return splitter.finish()
}

// step consumes the rune at i and reports how many further runes it took, so a
// multi-rune dollar tag is not re-read one character at a time.
func (s *sqlSplitter) step(runes []rune, i int) int {
	r := runes[i]

	if consumed, handled := s.stepInsideLiteral(runes, i); handled {
		return consumed
	}

	switch r {
	case '$':
		if !s.inSingle && !s.inDouble {
			if tag, width := readDollarTag(runes, i); tag != "" {
				s.current.WriteString(tag)
				s.dollarTag = tag
				return width - 1
			}
		}
	case '\'':
		if !s.inDouble {
			s.inSingle = !s.inSingle
		}
	case '"':
		if !s.inSingle {
			s.inDouble = !s.inDouble
		}
	case ';':
		if !s.inSingle && !s.inDouble {
			s.flush()
			return 0
		}
	}

	s.current.WriteRune(r)
	return 0
}

// stepInsideLiteral handles the runes that carry no syntax because a
// dollar-quoted block or a line comment is still open. It reports whether it
// took the rune, alongside how many further ones it consumed.
func (s *sqlSplitter) stepInsideLiteral(runes []rune, i int) (int, bool) {
	r := runes[i]

	if s.dollarTag != "" {
		if tag, width := readDollarTag(runes, i); tag == s.dollarTag {
			s.current.WriteString(tag)
			s.dollarTag = ""
			return width - 1, true
		}
		s.current.WriteRune(r)
		return 0, true
	}

	if s.inLineComment {
		if r == '\n' {
			s.inLineComment = false
		}
		s.current.WriteRune(r)
		return 0, true
	}
	if !s.inSingle && !s.inDouble && strings.HasSuffix(s.current.String(), "--") {
		s.inLineComment = true
		s.current.WriteRune(r)
		return 0, true
	}
	return 0, false
}

// flush ends the statement being built, dropping one that is only comments.
func (s *sqlSplitter) flush() {
	if statement := stripSQLComments(s.current.String()); statement != "" {
		s.statements = append(s.statements, statement)
	}
	s.current.Reset()
}

// finish takes the trailing statement, which a file need not end with a
// semicolon to have.
func (s *sqlSplitter) finish() []string {
	s.flush()
	return s.statements
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
