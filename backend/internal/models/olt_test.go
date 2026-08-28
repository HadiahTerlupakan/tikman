package models_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm/schema"
)

// The SQL migrations name these columns, GORM derives them from the field
// names, and nothing fails loudly when the two disagree: a read just returns
// the zero value. GORM turns VLANs into "vla_ns" unless the tag pins it, which
// is how the cached VLAN list came back empty from Postgres while the SQLite
// tests, which build their schema from these same structs, stayed green.
func TestOLTColumnNamesMatchTheMigrations(t *testing.T) {
	parsed, err := schema.Parse(&models.OLT{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	columns := make(map[string]string, len(parsed.Fields))
	for _, field := range parsed.Fields {
		columns[field.Name] = field.DBName
	}

	for field, want := range map[string]string{
		"VLANs":                  "vlans",
		"VLANsUpdatedAt":         "vlans_updated_at",
		"TCONTProfiles":          "tcont_profiles",
		"VLANProfiles":           "vlan_profiles",
		"ONUTypes":               "onu_types",
		"TCONTProfilesUpdatedAt": "tcont_profiles_updated_at",
		"SNMPCommunity":          "snmp_community",
		"DiscoveryPhase":         "discovery_phase",
		"DiscoveryStartedAt":     "discovery_started_at",
		"DiscoveryRegistered":    "discovery_registered",
	} {
		assert.Equal(t, want, columns[field], "field %s maps to the wrong column", field)
	}
}
