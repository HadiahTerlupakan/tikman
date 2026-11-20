package switchdriver

import (
	"context"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/connectivity/shell"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The only commands the Huawei CE driver sends. All are display commands;
// the shell itself adds "screen-length 0 temporary" at login.
const (
	ceVersion        = "display version"
	ceInterfaceBrief = "display interface brief"
	ceConfig         = "display current-configuration"
	ceSTPBrief       = "display stp brief"
	ceVLAN           = "display vlan"
	ceLLDPBrief      = "display lldp neighbor brief"
	ceMACTable       = "display mac-address"
)

func init() { Register(models.SwitchVendorHuaweiCE, openHuawei) }

// openShell is shell.Open behind a variable so tests can record what the
// driver sends without a device.
var openShell = func(ctx context.Context, cfg shell.Config) (shell.Shell, error) {
	session, err := shell.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// huaweiReader reads a CloudEngine switch running VRP over its CLI. The SNMP
// community is not used here: counters are polled elsewhere.
type huaweiReader struct{ sh shell.Shell }

func openHuawei(ctx context.Context, sw *models.Switch, _, password string) (Reader, error) {
	if sw.Protocol != models.SwitchProtocolTelnet && sw.Protocol != models.SwitchProtocolSSH {
		return nil, fmt.Errorf("switchdriver: Huawei CE is reached over telnet or ssh, not %q", sw.Protocol)
	}
	sh, err := openShell(ctx, shell.Config{
		Protocol:          shell.Protocol(sw.Protocol),
		Host:              sw.Host,
		Port:              sw.Port,
		Username:          sw.Username,
		Password:          password,
		PinnedFingerprint: sw.HostKeyFingerprint,
		Dialect:           shell.DialectVRP,
	})
	if err != nil {
		return nil, fmt.Errorf("switchdriver: open %s: %w", sw.Host, err)
	}
	return &huaweiReader{sh: sh}, nil
}

// Identify reads the model and software version.
func (r *huaweiReader) Identify(ctx context.Context) (Identity, error) {
	out, err := r.run(ctx, ceVersion)
	if err != nil {
		return Identity{}, err
	}
	return parseVersion(out)
}

// ReadPorts takes VLAN and STP settings from the running config, which is
// the source of truth on VRP, admin state from the interface table, and the
// effective edge state from STP.
func (r *huaweiReader) ReadPorts(ctx context.Context) ([]Port, error) {
	config, err := r.run(ctx, ceConfig)
	if err != nil {
		return nil, err
	}
	brief, err := r.run(ctx, ceInterfaceBrief)
	if err != nil {
		return nil, err
	}
	stp, err := r.run(ctx, ceSTPBrief)
	if err != nil {
		return nil, err
	}
	return mergePorts(parseConfigPorts(config), parseInterfaceBrief(brief), parseSTPBrief(stp)), nil
}

// ReadVLANs reads the VLANs and their names.
func (r *huaweiReader) ReadVLANs(ctx context.Context) ([]VLAN, error) {
	out, err := r.run(ctx, ceVLAN)
	if err != nil {
		return nil, err
	}
	return parseVLANs(out), nil
}

// ReadNeighbors reads LLDP neighbours, then the MAC table.
func (r *huaweiReader) ReadNeighbors(ctx context.Context) ([]Neighbor, error) {
	lldp, err := r.run(ctx, ceLLDPBrief)
	if err != nil {
		return nil, err
	}
	macs, err := r.run(ctx, ceMACTable)
	if err != nil {
		return nil, err
	}
	return append(parseLLDPBrief(lldp), parseMACTable(macs)...), nil
}

// Backup returns the running config as the switch prints it. Output that
// holds neither a sysname nor the closing "return" is not a config, and
// storing it would replace the last good backup with a message.
func (r *huaweiReader) Backup(ctx context.Context) (string, error) {
	config, err := r.run(ctx, ceConfig)
	if err != nil {
		return "", err
	}
	for _, line := range lines(config) {
		if strings.HasPrefix(line, "sysname ") || line == "return" {
			return config, nil
		}
	}
	return "", fmt.Errorf("switchdriver: %q printed no config", ceConfig)
}

// vrpRefused starts VRP's reply to a command it will not run: unknown, or
// above the user's command level.
const vrpRefused = "Error:"

// run sends one command. A refusal comes back with the prompt like any
// output, so the shell cannot see it; it is turned into an error here.
func (r *huaweiReader) run(ctx context.Context, cmd string) (string, error) {
	out, err := r.sh.Run(ctx, cmd)
	if err != nil {
		return "", err
	}
	for _, line := range lines(out) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, vrpRefused) {
			return "", fmt.Errorf("switchdriver: %q refused: %s", cmd, line)
		}
		break
	}
	return out, nil
}

// Close ends the session.
func (r *huaweiReader) Close() error { return r.sh.Close() }
