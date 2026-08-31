package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitSeparatesPlainStatements(t *testing.T) {
	statements := splitSQLStatements("CREATE TABLE a (id int);\nCREATE TABLE b (id int);")

	require.Len(t, statements, 2)
	require.Contains(t, statements[0], "CREATE TABLE a")
	require.Contains(t, statements[1], "CREATE TABLE b")
}

func TestSplitKeepsASemicolonInsideAString(t *testing.T) {
	statements := splitSQLStatements("INSERT INTO t (c) VALUES ('a;b');")

	require.Len(t, statements, 1)
	require.Contains(t, statements[0], "'a;b'")
}

func TestSplitKeepsADollarQuotedBlockWhole(t *testing.T) {
	// A DO block is one statement however many semicolons it contains. Splitting
	// inside it hands the database a fragment ending mid-block, which fails as
	// "unterminated dollar-quoted string" — and did.
	migration := `DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE column_name = 'old') THEN
        ALTER TABLE t RENAME COLUMN old TO new;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_t_old;`

	statements := splitSQLStatements(migration)

	require.Len(t, statements, 2)
	require.Contains(t, statements[0], "BEGIN")
	require.Contains(t, statements[0], "END IF")
	require.Contains(t, statements[0], "END $$")
	require.Contains(t, statements[1], "DROP INDEX")
}

func TestSplitKeepsATaggedDollarBlockWhole(t *testing.T) {
	// Postgres allows a tag between the dollars, and a block closes only on its
	// own tag. Treating any $...$ as a terminator would end the block early.
	migration := `DO $fix$ BEGIN PERFORM 1; END $fix$;
SELECT 2;`

	statements := splitSQLStatements(migration)

	require.Len(t, statements, 2)
	require.Contains(t, statements[0], "PERFORM 1")
	require.Contains(t, statements[0], "END $fix$")
	require.Contains(t, statements[1], "SELECT 2")
}

func TestSplitDoesNotMistakeADollarInsideAStringForAQuote(t *testing.T) {
	// A price or a regex can carry a dollar. Reading it as the start of a
	// dollar-quoted block would swallow the rest of the file.
	statements := splitSQLStatements("INSERT INTO t (c) VALUES ('$5');\nSELECT 1;")

	require.Len(t, statements, 2)
	require.Contains(t, statements[0], "'$5'")
	require.Contains(t, statements[1], "SELECT 1")
}

func TestSplitIgnoresASemicolonInALineComment(t *testing.T) {
	statements := splitSQLStatements("-- a comment; with a semicolon\nSELECT 1;")

	require.Len(t, statements, 1)
	require.Contains(t, statements[0], "SELECT 1")
}

func TestSplitReturnsNothingForCommentsAlone(t *testing.T) {
	require.Empty(t, splitSQLStatements("-- nothing but a comment\n"))
}

func TestSplitAcceptsAFinalStatementWithNoSemicolon(t *testing.T) {
	statements := splitSQLStatements("SELECT 1")

	require.Len(t, statements, 1)
}
