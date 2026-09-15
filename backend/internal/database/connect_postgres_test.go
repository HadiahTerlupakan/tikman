package database

import (
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A wrong password has to come back from the real driver as a refusal. If its
// error stopped wrapping the server's answer, every misconfigured deploy would
// sit through the whole retry window before saying anything.
func TestTheDriverReportsRejectedCredentialsAsARefusal(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the driver's refusal is then never checked")
		}
		t.Skip("set TEST_POSTGRES_DSN to check a refusal from a real Postgres")
	}
	parsed, err := pgconn.ParseConfig(dsn)
	require.NoError(t, err)

	_, err = open(fmt.Sprintf("host=%s port=%d user=%s password=not-the-password dbname=%s sslmode=disable",
		parsed.Host, parsed.Port, parsed.User, parsed.Database))

	require.Error(t, err)
	assert.False(t, notAnsweringYet(err), "a rejected password must not be retried: %v", err)
}
