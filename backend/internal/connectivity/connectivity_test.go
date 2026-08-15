package connectivity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPingTest_Success(t *testing.T) {
	// Test with localhost (should always be reachable)
	err := PingTest("127.0.0.1", 2*time.Second)
	assert.NoError(t, err)
}

func TestPingTest_Timeout(t *testing.T) {
	// Test with unreachable IP
	err := PingTest("192.0.2.1", 100*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}
