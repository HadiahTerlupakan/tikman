package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestMaskConfigHidesEverySecret(t *testing.T) {
	cases := []struct {
		name   string
		vendor models.SwitchVendor
		line   string
		want   string
		secret string
	}{
		{"CE SNMP community", models.SwitchVendorHuaweiCE,
			"snmp-agent community read cipher %^%#x", "snmp-agent community read cipher ***", "%^%#x"},
		{"CE SNMP community with acl", models.SwitchVendorHuaweiCE,
			"snmp-agent community write cipher %^%#w%^%# acl 2000", "snmp-agent community write cipher *** acl 2000", "%^%#w"},
		{"CE local user", models.SwitchVendorHuaweiCE,
			" local-user a password irreversible-cipher $1a$abc$", " local-user a password irreversible-cipher ***", "$1a$abc$"},
		{"CE radius shared key", models.SwitchVendorHuaweiCE,
			" radius-server shared-key cipher %^%#r%^%#", " radius-server shared-key cipher ***", "%^%#r"},
		{"CE VTY password", models.SwitchVendorHuaweiCE,
			" set authentication password cipher $1a$vty$", " set authentication password cipher ***", "$1a$vty$"},
		{"CE trap target", models.SwitchVendorHuaweiCE,
			"snmp-agent target-host trap address udp-domain 192.0.2.5 params securityname cipher %^%#t%^%# v2c",
			"snmp-agent target-host trap address udp-domain 192.0.2.5 params securityname cipher *** v2c", "%^%#t"},
		{"CE trap target in clear", models.SwitchVendorHuaweiCE,
			"snmp-agent target-host trap address udp-domain 192.0.2.5 params securityname public v2c",
			"snmp-agent target-host trap address udp-domain 192.0.2.5 params securityname *** v2c", "public"},
		{"CE NTP key no rule knows", models.SwitchVendorHuaweiCE,
			"ntp authentication-keyid 1 authentication-mode md5 cipher %^%#abc", "ntp ***", "%^%#abc"},
		{"CE cipher with a trailing word the rule does not know", models.SwitchVendorHuaweiCE,
			"snmp-agent community read cipher %^%#q%^%# extra", "snmp-agent ***", "%^%#q"},
		{"FiberHome SNMP community", models.SwitchVendorFiberHome,
			"snmp-server community x ro", "snmp-server ***", " x "},
		{"FiberHome user", models.SwitchVendorFiberHome,
			"username a password 0 x", "username ***", " x"},
		{"CRS SNMP community", models.SwitchVendorCRS,
			"/snmp community add name=x", "/snmp ***", "name=x"},
		{"single-token key=value line", models.SwitchVendorCRS,
			"    encryption-password=bar", "    ***", "bar"},
		{"tab-separated line", models.SwitchVendorFiberHome,
			"snmp\tcommunity x", "snmp ***", "community x"},
		{"secret in the first word", models.SwitchVendorHuaweiCE,
			" %^%#abc%^%#", " ***", "%^%#abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskConfig(tc.vendor, tc.line)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, tc.secret)
		})
	}
}

func TestMaskConfigKeepsHarmlessLines(t *testing.T) {
	for _, line := range []string{
		"interface 10GE1/0/1",
		" port trunk allow-pass vlan 10 20 30",
		" authentication-scheme default",
		" authentication-mode aaa",
		"ssh server cipher aes256_ctr aes128_ctr",
		"ssh server key-exchange dh_group_exchange_sha256",
	} {
		assert.Equal(t, line, MaskConfig(models.SwitchVendorHuaweiCE, line))
	}
}

func TestMaskConfigKeepsLineStructure(t *testing.T) {
	raw := "#\r\nsysname SW-TEST\r\n#\r\nsnmp-agent community read cipher %^%#x\r\n#\r\n"

	got := MaskConfig(models.SwitchVendorHuaweiCE, raw)

	assert.Equal(t, "#\nsysname SW-TEST\n#\nsnmp-agent community read cipher ***\n#\n", got)
	assert.False(t, strings.Contains(got, "%^%#"))
}
