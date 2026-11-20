package switchdriver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tikman/olt-provisioning/internal/connectivity/shell"
	"github.com/tikman/olt-provisioning/internal/models"
)

// fakeShell answers each command from a fixture and records every line sent.
type fakeShell struct {
	outputs map[string]string
	sent    []string
	closed  bool
}

func (f *fakeShell) Run(_ context.Context, cmd string) (string, error) {
	f.sent = append(f.sent, cmd)
	out, ok := f.outputs[cmd]
	if !ok {
		return "", errors.New("unexpected command " + cmd)
	}
	return out, nil
}

func (f *fakeShell) Close() error {
	f.closed = true
	return nil
}

func newFakeCE(t *testing.T) *fakeShell {
	return &fakeShell{outputs: map[string]string{
		"display version":               fixture(t, "version.txt"),
		"display interface brief":       fixture(t, "interface-brief.txt"),
		"display current-configuration": fixture(t, "current-configuration.txt"),
		"display stp brief":             fixture(t, "stp-brief.txt"),
		"display vlan":                  fixture(t, "vlan.txt"),
		"display lldp neighbor brief":   fixture(t, "lldp-neighbor-brief.txt"),
		"display mac-address":           fixture(t, "mac-address.txt"),
	}}
}

// useFakeShell routes the driver's shell.Open to fake and returns the config
// it was opened with.
func useFakeShell(t *testing.T, fake *fakeShell) *shell.Config {
	t.Helper()
	var opened shell.Config
	previous := openShell
	openShell = func(_ context.Context, cfg shell.Config) (shell.Shell, error) {
		opened = cfg
		return fake, nil
	}
	t.Cleanup(func() { openShell = previous })
	return &opened
}

func testCE(protocol models.SwitchProtocol) *models.Switch {
	return &models.Switch{
		Vendor: models.SwitchVendorHuaweiCE, Host: "192.0.2.10", Protocol: protocol, Port: 23,
		Username: "reader", HostKeyFingerprint: "SHA256:invented",
	}
}

func TestOpenHuaweiUsesVRPOverTheSwitchProtocol(t *testing.T) {
	for _, protocol := range []models.SwitchProtocol{models.SwitchProtocolTelnet, models.SwitchProtocolSSH} {
		opened := useFakeShell(t, newFakeCE(t))
		reader, err := Open(context.Background(), testCE(protocol), "public", "s3cret")
		require.NoError(t, err)
		assert.Equal(t, shell.Config{
			Protocol: shell.Protocol(protocol), Host: "192.0.2.10", Port: 23,
			Username: "reader", Password: "s3cret", PinnedFingerprint: "SHA256:invented",
			Dialect: shell.DialectVRP,
		}, *opened)
		require.NoError(t, reader.Close())
	}
}

func TestOpenHuaweiRefusesREST(t *testing.T) {
	useFakeShell(t, newFakeCE(t))
	_, err := Open(context.Background(), testCE(models.SwitchProtocolREST), "", "s3cret")
	assert.Error(t, err)
}

func TestOpenPassesShellErrorsThrough(t *testing.T) {
	previous := openShell
	openShell = func(context.Context, shell.Config) (shell.Shell, error) {
		return nil, shell.ErrHostKeyUnpinned{Presented: "SHA256:new"}
	}
	t.Cleanup(func() { openShell = previous })

	_, err := Open(context.Background(), testCE(models.SwitchProtocolSSH), "", "s3cret")
	var unpinned shell.ErrHostKeyUnpinned
	require.ErrorAs(t, err, &unpinned)
	assert.Equal(t, "SHA256:new", unpinned.Presented)
}

func TestOpenRefusesUnregisteredVendor(t *testing.T) {
	sw := testCE(models.SwitchProtocolSSH)
	sw.Vendor = models.SwitchVendorFiberHome
	_, err := Open(context.Background(), sw, "", "s3cret")
	assert.ErrorIs(t, err, ErrVendorNotSupported)
}

// Review Focus 1: a read must never send anything that could change the
// switch. Every line the driver sends is checked against the allow-list.
func TestHuaweiReaderSendsOnlyReadCommands(t *testing.T) {
	fake := newFakeCE(t)
	useFakeShell(t, fake)
	reader, err := Open(context.Background(), testCE(models.SwitchProtocolTelnet), "", "s3cret")
	require.NoError(t, err)
	ctx := context.Background()

	identity, err := reader.Identify(ctx)
	require.NoError(t, err)
	assert.Equal(t, "CE6850-48S4Q-EI", identity.Model)
	ports, err := reader.ReadPorts(ctx)
	require.NoError(t, err)
	assert.Len(t, ports, 12)
	vlans, err := reader.ReadVLANs(ctx)
	require.NoError(t, err)
	assert.Len(t, vlans, 5)
	neighbors, err := reader.ReadNeighbors(ctx)
	require.NoError(t, err)
	assert.Len(t, neighbors, 6)
	backup, err := reader.Backup(ctx)
	require.NoError(t, err)
	assert.Equal(t, fixture(t, "current-configuration.txt"), backup)
	require.NoError(t, reader.Close())
	assert.True(t, fake.closed)

	require.NotEmpty(t, fake.sent)
	for _, line := range fake.sent {
		allowed := strings.HasPrefix(line, "display ") || line == "screen-length 0 temporary"
		assert.True(t, allowed, "sent a command outside the read allow-list: %q", line)
	}
}

func TestHuaweiReaderReturnsShellErrors(t *testing.T) {
	fake := &fakeShell{outputs: map[string]string{}}
	useFakeShell(t, fake)
	reader, err := Open(context.Background(), testCE(models.SwitchProtocolTelnet), "", "s3cret")
	require.NoError(t, err)
	_, err = reader.ReadPorts(context.Background())
	assert.Error(t, err)
	_, err = reader.ReadNeighbors(context.Background())
	assert.Error(t, err)
}

// A VRP user below the command's level gets "Error: …" and the prompt back,
// which the shell sees as a finished command. The driver must report it, or
// the poller stores the error line as a config or wipes the VLANs it held.
func TestHuaweiReaderReportsARefusedCommand(t *testing.T) {
	refused := "Error: Unrecognized command found at '^' position."
	ctx := context.Background()
	for _, cmd := range []string{ceVersion, ceConfig, ceVLAN, ceLLDPBrief} {
		fake := newFakeCE(t)
		fake.outputs[cmd] = "\n" + refused
		useFakeShell(t, fake)
		reader, err := Open(ctx, testCE(models.SwitchProtocolTelnet), "", "s3cret")
		require.NoError(t, err)

		_, identifyErr := reader.Identify(ctx)
		_, portsErr := reader.ReadPorts(ctx)
		_, vlansErr := reader.ReadVLANs(ctx)
		_, neighborsErr := reader.ReadNeighbors(ctx)
		_, backupErr := reader.Backup(ctx)

		affected := map[string][]error{
			ceVersion: {identifyErr}, ceConfig: {portsErr, backupErr},
			ceVLAN: {vlansErr}, ceLLDPBrief: {neighborsErr},
		}[cmd]
		for _, err := range affected {
			require.Error(t, err, cmd)
			assert.Contains(t, err.Error(), refused, cmd)
		}
	}
}

func TestHuaweiBackupRefusesOutputThatIsNotAConfig(t *testing.T) {
	fake := newFakeCE(t)
	fake.outputs[ceConfig] = "Info: The system is busy. Please try again later."
	useFakeShell(t, fake)
	reader, err := Open(context.Background(), testCE(models.SwitchProtocolTelnet), "", "s3cret")
	require.NoError(t, err)

	_, err = reader.Backup(context.Background())

	assert.Error(t, err)
}
