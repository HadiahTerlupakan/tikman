package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func createHandlerTestSite(t *testing.T, db *gorm.DB, name string) models.Site {
	site := models.Site{Name: name}
	require.NoError(t, db.Create(&site).Error)
	return site
}

func TestSaveServerCreatesConfigurationOnFirstCall(t *testing.T) {
	handler, _, _ := SetupWireGuardHandlerTest(t)

	w, c := SetupTestContext(http.MethodPut, "/api/v1/wireguard/server", SaveWireguardServerRequest{
		EndpointHost: "vpn.contoh.id",
		ListenPort:   51820,
	})
	handler.SaveServer(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response WireguardServerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "vpn.contoh.id", response.EndpointHost)
	require.NotEmpty(t, response.PublicKey)
}

func TestGetServerReportsNotConfigured(t *testing.T) {
	handler, _, _ := SetupWireGuardHandlerTest(t)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/server", nil)
	handler.GetServer(c)

	require.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "NOT_CONFIGURED", response.Code)
}

func TestCreatePeerReturnsPeerWithoutPrivateKey(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")

	w, c := SetupTestContext(http.MethodPost, "/api/v1/wireguard/peers", CreateWireguardPeerRequest{
		SiteID:     site.ID.String(),
		Name:       "Site A",
		AllowedIPs: []string{"10.10.10.0/24"},
	})
	handler.CreatePeer(c)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotContains(t, w.Body.String(), "private",
		"a peer response must never expose key material")

	var response WireguardPeerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "10.88.0.2", response.TunnelAddress)
	require.False(t, response.Connected)
}

func TestCreatePeerRejectsOverlapWithReadableMessage(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	first := createHandlerTestSite(t, db, "Site Bandung")
	second := createHandlerTestSite(t, db, "Site Bogor")

	_, err = service.CreatePeer(first.ID, "Site Bandung", []string{"192.168.1.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodPost, "/api/v1/wireguard/peers", CreateWireguardPeerRequest{
		SiteID:     second.ID.String(),
		Name:       "Site Bogor",
		AllowedIPs: []string{"192.168.1.0/24"},
	})
	handler.CreatePeer(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Site Bandung")
}

func TestListPeersMarksConnectionState(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	_, err = service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/peers", nil)
	handler.ListPeers(c)

	require.Equal(t, http.StatusOK, w.Code)

	var peers []WireguardPeerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &peers))
	require.Len(t, peers, 1)
	require.False(t, peers[0].Connected, "a peer that never handshook must read as disconnected")
}

func TestGetPeerConfigReturnsRequestedFormat(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/peers/"+peer.ID.String()+"/config?format=mikrotik", nil)
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.GetPeerConfig(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response WireguardPeerConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "mikrotik", response.Format)
	require.Contains(t, response.Config, "/interface/wireguard/add")
}

func TestGetPeerConfigDefaultsToWGQuick(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/peers/"+peer.ID.String()+"/config", nil)
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.GetPeerConfig(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "[Interface]")
}

func TestSuggestSubnetsUsesRegisteredOLTs(t *testing.T) {
	handler, _, db := SetupWireGuardHandlerTest(t)
	site := createHandlerTestSite(t, db, "Site A")
	require.NoError(t, db.Create(&models.OLT{
		SiteID: site.ID, Name: "OLT 1", IPAddress: "10.10.10.5",
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}).Error)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/sites/"+site.ID.String()+"/suggested-subnets", nil)
	c.Params = gin.Params{{Key: "site_id", Value: site.ID.String()}}
	handler.SuggestSubnets(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response SuggestedSubnetsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, []string{"10.10.10.0/24"}, response.Subnets)
}

func TestDeletePeerReturnsNoContent(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodDelete, "/api/v1/wireguard/peers/"+peer.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.DeletePeer(c)

	require.Equal(t, http.StatusNoContent, w.Code)

	var count int64
	require.NoError(t, db.Model(&models.WireGuardPeer{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdatePeerTogglesEnabled(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	disabled := false
	w, c := SetupTestContext(http.MethodPut, "/api/v1/wireguard/peers/"+peer.ID.String(), UpdateWireguardPeerRequest{
		Enabled: &disabled,
	})
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.UpdatePeer(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response WireguardPeerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.False(t, response.Enabled)
	require.False(t, response.Connected, "a disabled peer is never reported as connected")
	require.NotContains(t, w.Body.String(), "private")
}

func TestUpdatePeerReportsUnknownPeerAsNotFound(t *testing.T) {
	handler, service, _ := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)

	missing := uuid.New().String()
	name := "Site A"
	w, c := SetupTestContext(http.MethodPut, "/api/v1/wireguard/peers/"+missing, UpdateWireguardPeerRequest{
		Name: &name,
	})
	c.Params = gin.Params{{Key: "id", Value: missing}}
	handler.UpdatePeer(c)

	require.Equal(t, http.StatusNotFound, w.Code,
		"a peer that does not exist must not read as bad input")
}

func TestCreatePeerReportsMissingServerAsBadRequest(t *testing.T) {
	handler, _, db := SetupWireGuardHandlerTest(t)
	site := createHandlerTestSite(t, db, "Site A")

	w, c := SetupTestContext(http.MethodPost, "/api/v1/wireguard/peers", CreateWireguardPeerRequest{
		SiteID:     site.ID.String(),
		Name:       "Site A",
		AllowedIPs: []string{"10.10.10.0/24"},
	})
	handler.CreatePeer(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "NOT_CONFIGURED")
}

func TestTestReachabilityExplainsAnAddressOutsideTheSubnet(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodPost, "/api/v1/wireguard/peers/"+peer.ID.String()+"/test",
		TestReachabilityRequest{Address: "192.168.1.5"})
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.TestReachability(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response ReachabilityResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.False(t, response.Routed)
	require.False(t, response.Reachable)
	// Naming both the address and the configured subnet is the whole point: the
	// operator cannot tell which of the two is the mistake otherwise.
	require.Contains(t, response.Message, "192.168.1.5")
	require.Contains(t, response.Message, "10.10.10.0/24")
}

func TestTestReachabilityRejectsSomethingThatIsNotAnAddress(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodPost, "/api/v1/wireguard/peers/"+peer.ID.String()+"/test",
		TestReachabilityRequest{Address: "bukan-ip"})
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.TestReachability(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "INVALID_CONFIGURATION")
}
