package services

import (
	"regexp"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
)

// maskedValue replaces every secret in a stored backup.
const maskedValue = "***"

// sensitiveWords mark a line that may hold a secret. "securityname" is not in
// the spec's list: a CE trap target names its community there, and on older
// VRP releases it is shown in clear, with none of the other words on the line.
var sensitiveWords = []string{
	"password", "cipher", "key", "secret", "community", "usm-user", "%^%#", "authentication", "securityname",
}

// maskRule is a line shape known to be safe to keep, apart from the secret it
// holds. A rule with a replacement keeps groups 1 and 2 around the secret; a
// rule without one keeps the line whole. Every rule is anchored at both ends
// and lists what may follow the secret, so an unexpected word on the line
// falls through to masking the whole line.
type maskRule struct {
	re          *regexp.Regexp
	replacement string
}

func keepSecretAround(pattern string) maskRule {
	return maskRule{re: regexp.MustCompile(pattern), replacement: "${1}" + maskedValue + "${2}"}
}

func keepWhole(pattern string) maskRule {
	return maskRule{re: regexp.MustCompile(pattern)}
}

// ceMaskRules cover the secret-bearing lines of VRP's current-configuration,
// and the lines whose words only look sensitive.
var ceMaskRules = []maskRule{
	keepSecretAround(`^(\s*snmp-agent community (?:read|write) (?:cipher )?)\S+((?: (?:acl|mib-view) \S+)*)\s*$`),
	keepSecretAround(`^(\s*snmp-agent target-host (?:trap|inform) address udp-domain [0-9A-Fa-f.:]+(?: udp-port \d+)?` +
		` params securityname (?:cipher )?)\S+((?: (?:v1|v2c|v3|private-netmanager|ext-vb))*)\s*$`),
	keepSecretAround(`^(\s*local-user \S+ password (?:irreversible-cipher |cipher |simple )?)\S+()\s*$`),
	keepSecretAround(`^(\s*radius-server shared-key (?:cipher |simple )?)\S+()\s*$`),
	keepSecretAround(`^(\s*set authentication password (?:cipher |simple )?)\S+()\s*$`),
	keepWhole(`^\s*authentication-scheme \S+\s*$`),
	keepWhole(`^\s*authentication-mode (?:aaa|password|none)\s*$`),
	keepWhole(`^\s*ssh server (?:cipher|hmac|key-exchange)(?: [a-z0-9_]+)+\s*$`),
}

// maskRules holds each vendor's rules. A vendor with none still has every
// sensitive line masked, from its first argument onward.
var maskRules = map[models.SwitchVendor][]maskRule{
	models.SwitchVendorHuaweiCE: ceMaskRules,
}

// MaskConfig returns a config with every secret replaced by ***. It is
// deny-by-default: a line holding a sensitive word that no rule of the vendor
// recognises keeps only its indentation and first word, or only its
// indentation when that word could itself hold the secret.
func MaskConfig(vendor models.SwitchVendor, raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = maskLine(maskRules[vendor], strings.TrimSuffix(line, "\r"))
	}
	return strings.Join(lines, "\n")
}

func maskLine(rules []maskRule, line string) string {
	if !isSensitive(line) {
		return line
	}
	for _, rule := range rules {
		if !rule.re.MatchString(line) {
			continue
		}
		if rule.replacement == "" {
			return line
		}
		return rule.re.ReplaceAllString(line, rule.replacement)
	}
	return maskFromFirstArgument(line)
}

func isSensitive(line string) bool {
	lower := strings.ToLower(line)
	for _, word := range sensitiveWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

// maskFromFirstArgument keeps the first word only when it cannot itself be the
// secret: a key=value line or a sensitive first word is masked whole.
func maskFromFirstArgument(line string) string {
	body := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(body)]
	fields := strings.Fields(body)
	if len(fields) == 0 || strings.Contains(fields[0], "=") || isSensitive(fields[0]) {
		return indent + maskedValue
	}
	return indent + fields[0] + " " + maskedValue
}
