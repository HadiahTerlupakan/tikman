package connectivity

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func skipIfPingPermissionDenied(t *testing.T, err error) bool {
	if err != nil && strings.Contains(err.Error(), "permission denied") {
		t.Skipf("skipping ping test in unprivileged CI environment: %v", err)
		return true
	}

	return false
}

func TestPingTest_Success(t *testing.T) {
	// macOS never delivers the echo reply to the unprivileged UDP socket that
	// PingTest uses when the target is the loopback interface; pinging a real
	// host from the same socket works. Linux, which CI and the container image
	// run, has no such gap.
	if runtime.GOOS == "darwin" {
		t.Skip("skipping ping test: macOS drops unprivileged ICMP on loopback")
	}

	// Test with localhost (should always be reachable)
	err := PingTest("127.0.0.1", 2*time.Second)
	if skipIfPingPermissionDenied(t, err) {
		return
	}
	assert.NoError(t, err)
}

func TestPingTest_Timeout(t *testing.T) {
	// Test with unreachable IP
	err := PingTest("192.0.2.1", 100*time.Millisecond)
	if skipIfPingPermissionDenied(t, err) {
		return
	}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestSSHTest_InvalidHost(t *testing.T) {
	err := SSHTest("192.0.2.1", 22, "testuser", "testpass", 1*time.Second)
	assert.Error(t, err)
	// Accept either timeout or no route to host (both indicate unreachable)
	errMsg := err.Error()
	hasTimeout := strings.Contains(errMsg, "timeout")
	hasNoRoute := strings.Contains(errMsg, "no route to host")
	assert.True(t, hasTimeout || hasNoRoute, "expected timeout or no route to host error, got: %s", errMsg)
}

func TestSSHTest_InvalidPort(t *testing.T) {
	err := SSHTest("127.0.0.1", 9999, "testuser", "testpass", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestTelnetTest_InvalidHost(t *testing.T) {
	err := TelnetTest("192.0.2.1", 23, "testuser", "testpass", 1*time.Second)
	assert.Error(t, err)
}

func TestTelnetTest_InvalidPort(t *testing.T) {
	err := TelnetTest("127.0.0.1", 9999, "testuser", "testpass", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestSNMPTest_InvalidHost(t *testing.T) {
	err := SNMPTest("192.0.2.1", 161, "public", 1*time.Second)
	assert.Error(t, err)
	// Accept either timeout or no route to host (both indicate unreachable)
	errMsg := err.Error()
	hasTimeout := strings.Contains(errMsg, "timeout")
	hasNoRoute := strings.Contains(errMsg, "no route to host")
	assert.True(t, hasTimeout || hasNoRoute, "expected timeout or no route to host error, got: %s", errMsg)
}

func TestSNMPTest_InvalidPort(t *testing.T) {
	err := SNMPTest("127.0.0.1", 9999, "public", 1*time.Second)
	assert.Error(t, err)
}
