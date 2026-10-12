package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

const routerTestKey = "12345678901234567890123456789012"

var testFingerprint = strings.Repeat("ab", 32)

func newRouterServiceForTest(t *testing.T) (*RouterService, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	return NewRouterService(db, routerTestKey), db
}

func validRouterInput() RouterInput {
	return RouterInput{
		Name: "Border APJII", Host: "192.0.2.1", SNMPCommunity: "community-secret",
		APIUsername: "tikman", APIPassword: "password-secret", TLSFingerprint: testFingerprint,
	}
}

func TestCreateRouterEncryptsSecretsAndSchedulesEveryTier(t *testing.T) {
	service, db := newRouterServiceForTest(t)

	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	var stored models.MikrotikRouter
	require.NoError(t, db.First(&stored, "id = ?", router.ID).Error)
	assert.NotEqual(t, "community-secret", stored.SNMPCommunity)
	assert.NotEqual(t, "password-secret", stored.APIPassword)
	assert.Equal(t, DefaultRouterSNMPPort, stored.SNMPPort)
	assert.Equal(t, DefaultRouterAPIPort, stored.APIPort)
	assert.True(t, stored.Enabled)
	assert.Equal(t, models.RouterStatusUnknown, stored.Status)

	community, password, err := service.Secrets(&stored)
	require.NoError(t, err)
	assert.Equal(t, "community-secret", community)
	assert.Equal(t, "password-secret", password)

	var kinds []string
	require.NoError(t, db.Model(&models.RouterPollJob{}).Where("router_id = ?", router.ID).Pluck("kind", &kinds).Error)
	assert.ElementsMatch(t, []string{"interfaces", "bgp", "discovery"}, kinds)
}

func TestCreateRouterRejectsIncompleteInput(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	cases := map[string]func(*RouterInput){
		"no name":           func(in *RouterInput) { in.Name = "  " },
		"no host":           func(in *RouterInput) { in.Host = "" },
		"no community":      func(in *RouterInput) { in.SNMPCommunity = "" },
		"no password":       func(in *RouterInput) { in.APIPassword = "" },
		"short fingerprint": func(in *RouterInput) { in.TLSFingerprint = "abcd" },
		"not hex":           func(in *RouterInput) { in.TLSFingerprint = strings.Repeat("zz", 32) },
	}
	for name, mutate := range cases {
		in := validRouterInput()
		mutate(&in)
		_, err := service.Create(in)
		assert.True(t, errors.Is(err, ErrValidation), "%s: got %v", name, err)
	}
}

func TestUpdateRouterKeepsSecretsLeftBlank(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	name, blank, port := "Border baru", "", 18443
	updated, err := service.Update(router.ID, RouterUpdate{Name: &name, APIPassword: &blank, APIPort: &port})
	require.NoError(t, err)
	assert.Equal(t, "Border baru", updated.Name)
	assert.Equal(t, 18443, updated.APIPort)

	var stored models.MikrotikRouter
	require.NoError(t, db.First(&stored, "id = ?", router.ID).Error)
	_, password, err := service.Secrets(&stored)
	require.NoError(t, err)
	assert.Equal(t, "password-secret", password)
}

func TestUpdateRouterCanDisableIt(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	off := false
	updated, err := service.Update(router.ID, RouterUpdate{Enabled: &off})
	require.NoError(t, err)
	assert.False(t, updated.Enabled)
}

func TestUpdateRouterRejectsAnEmptyName(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	empty := " "
	_, err = service.Update(router.ID, RouterUpdate{Name: &empty})
	assert.True(t, errors.Is(err, ErrValidation))
}

func TestDeleteRouterReportsAnUnknownOne(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	err := service.Delete(uuid.New())
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestListRoutersIsOrderedByName(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	for _, name := range []string{"Zeta", "Alpha"} {
		in := validRouterInput()
		in.Name = name
		_, err := service.Create(in)
		require.NoError(t, err)
	}

	routers, err := service.List()
	require.NoError(t, err)
	require.Len(t, routers, 2)
	assert.Equal(t, "Alpha", routers[0].Name)
}
