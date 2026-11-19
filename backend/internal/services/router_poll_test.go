package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// fakeRouterAPI answers like a router; err, when set, is returned by every call.
type fakeRouterAPI struct {
	resource    mikrotik.SystemResource
	clock       mikrotik.Clock
	interfaces  []mikrotik.Interface
	vlans       []mikrotik.VLAN
	addresses   []mikrotik.Address
	connections []mikrotik.BGPConnection
	sessions    []mikrotik.BGPSession
	routes      []mikrotik.Route
	routesErr   error
	// bgpRoutes answers RoutesTo by exact prefix; asked records the prefixes.
	bgpRoutes map[string][]mikrotik.BGPRoute
	asked     []string
	// onRoutesTo, when set, runs inside the first RoutesTo call, so a test
	// can hold a lookup open.
	onRoutesTo  func()
	rules       []mikrotik.FilterRule
	lists       map[string][]mikrotik.AddressListEntry
	listsErr    error
	pppoe       []mikrotik.PPPoEServer
	pppoeErr    error
	defaults    []mikrotik.Route
	defaultsErr error
	// calls records the BRAS-only reads, so a test can show a border router
	// never makes them.
	calls  []string
	err    error
	closed int
}

func (f *fakeRouterAPI) Close() error { f.closed++; return nil }

func (f *fakeRouterAPI) SystemResource() (mikrotik.SystemResource, error) { return f.resource, f.err }
func (f *fakeRouterAPI) Clock() (mikrotik.Clock, error)                   { return f.clock, f.err }
func (f *fakeRouterAPI) Interfaces() ([]mikrotik.Interface, error)        { return f.interfaces, f.err }
func (f *fakeRouterAPI) VLANs() ([]mikrotik.VLAN, error)                  { return f.vlans, f.err }
func (f *fakeRouterAPI) Addresses() ([]mikrotik.Address, error)           { return f.addresses, f.err }
func (f *fakeRouterAPI) BGPConnections() ([]mikrotik.BGPConnection, error) {
	return f.connections, f.err
}
func (f *fakeRouterAPI) BGPSessions() ([]mikrotik.BGPSession, error) { return f.sessions, f.err }
func (f *fakeRouterAPI) StaticRoutes() ([]mikrotik.Route, error)     { return f.routes, f.routesErr }
func (f *fakeRouterAPI) FilterRules() ([]mikrotik.FilterRule, error) { return f.rules, f.err }
func (f *fakeRouterAPI) AddressList(list string) ([]mikrotik.AddressListEntry, error) {
	if f.listsErr != nil {
		return nil, f.listsErr
	}
	return f.lists[list], f.err
}
func (f *fakeRouterAPI) RoutesTo(prefix string) ([]mikrotik.BGPRoute, error) {
	f.asked = append(f.asked, prefix)
	if f.onRoutesTo != nil {
		hook := f.onRoutesTo
		f.onRoutesTo = nil
		hook()
	}
	return f.bgpRoutes[prefix], f.err
}

func (f *fakeRouterAPI) PPPoEServers() ([]mikrotik.PPPoEServer, error) {
	if f.pppoeErr != nil {
		return nil, f.pppoeErr
	}
	return f.pppoe, f.err
}
func (f *fakeRouterAPI) PPPActiveCount() (int, error)        { return 0, f.err }
func (f *fakeRouterAPI) IPPools() ([]mikrotik.IPPool, error) { return nil, f.err }
func (f *fakeRouterAPI) PoolUsedCount(string) (int, error)   { return 0, f.err }
func (f *fakeRouterAPI) RadiusServers() ([]mikrotik.RadiusServer, error) {
	return nil, f.err
}
func (f *fakeRouterAPI) RadiusMonitor(string) (mikrotik.RadiusStats, error) {
	return mikrotik.RadiusStats{}, f.err
}
func (f *fakeRouterAPI) CPULoads() ([]mikrotik.CPULoad, error)          { return nil, f.err }
func (f *fakeRouterAPI) DynamicQueues() ([]mikrotik.SimpleQueue, error) { return nil, f.err }
func (f *fakeRouterAPI) DefaultRoute() ([]mikrotik.Route, error) {
	f.calls = append(f.calls, "DefaultRoute")
	if f.defaultsErr != nil {
		return nil, f.defaultsErr
	}
	return f.defaults, f.err
}

type pollFixture struct {
	service   *RouterPollService
	db        *gorm.DB
	router    *models.MikrotikRouter
	api       *fakeRouterAPI
	counters  []connectivity.IfCounters
	snmpErr   error
	gauges    []connectivity.Gauge
	gaugeErr  error
	ifNames   map[string]int
	ifNameErr error
	openErr   error
}

func newPollFixture(t *testing.T) *pollFixture {
	t.Helper()
	routers, db := newRouterServiceForTest(t)
	router, err := routers.Create(validRouterInput())
	require.NoError(t, err)

	f := &pollFixture{db: db, router: router, api: &fakeRouterAPI{clock: mikrotik.Clock{GMTOffset: "+07:00"}}}
	f.service = &RouterPollService{
		db:      db,
		routers: routers,
		newAPI: func(*models.MikrotikRouter, string) (RouterAPI, error) {
			if f.openErr != nil {
				return nil, f.openErr
			}
			return f.api, nil
		},
		readCounters: func(context.Context, string, int, string) ([]connectivity.IfCounters, error) {
			return f.counters, f.snmpErr
		},
		readGauges: func(context.Context, string, int, string) ([]connectivity.Gauge, error) {
			return f.gauges, f.gaugeErr
		},
		readIfNames: func(context.Context, string, int, string) (map[string]int, error) {
			f.api.calls = append(f.api.calls, "WalkIfNames")
			return f.ifNames, f.ifNameErr
		},
		fetchFingerprint: func(string, int) (string, error) { return "ef" + testFingerprint[2:], nil },
	}
	return f
}

func (f *pollFixture) addInterface(t *testing.T, name, addresses string, disabled bool) models.RouterInterface {
	t.Helper()
	iface := models.RouterInterface{RouterID: f.router.ID, Name: name, Type: "vlan", Addresses: addresses,
		Disabled: disabled, OperStatus: "unknown"}
	require.NoError(t, f.db.Create(&iface).Error)
	return iface
}

func TestPollInterfacesRecordsARateFromTheSecondSample(t *testing.T) {
	f := newPollFixture(t)
	iface := f.addInterface(t, "vlan-transit", "", false)
	start := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

	f.counters = []connectivity.IfCounters{{IfIndex: 19, Name: "vlan-transit", InOctets: 1_000, OutOctets: 2_000, SpeedMbps: 10000, OperUp: true}}
	require.NoError(t, f.service.PollInterfaces(f.router, start))

	var samples int64
	f.db.Model(&models.RouterInterfaceMetric{}).Count(&samples)
	assert.Zero(t, samples, "one counter reading is not a rate")

	f.counters[0].InOctets += 750_000_000
	f.counters[0].OutOctets += 75_000_000
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(time.Minute)))

	var metric models.RouterInterfaceMetric
	require.NoError(t, f.db.First(&metric, "interface_id = ?", iface.ID).Error)
	assert.InDelta(t, 100_000_000, metric.InBps, 0.01)
	assert.InDelta(t, 10_000_000, metric.OutBps, 0.01)

	var stored models.RouterInterface
	require.NoError(t, f.db.First(&stored, "id = ?", iface.ID).Error)
	require.NotNil(t, stored.IfIndex)
	assert.Equal(t, 19, *stored.IfIndex)
	assert.Equal(t, "up", stored.OperStatus)
	require.NotNil(t, stored.LastInBps)
	assert.InDelta(t, 100_000_000, *stored.LastInBps, 0.01)

	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterStatusUp, router.Status)
}

func TestPollInterfacesDropsTheSampleAfterACounterReset(t *testing.T) {
	f := newPollFixture(t)
	f.addInterface(t, "vlan-transit", "", false)
	start := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

	f.counters = []connectivity.IfCounters{{Name: "vlan-transit", InOctets: 9_000_000, OutOctets: 9_000_000}}
	require.NoError(t, f.service.PollInterfaces(f.router, start))
	f.counters[0].InOctets, f.counters[0].OutOctets = 500, 500
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(time.Minute)))

	var samples int64
	f.db.Model(&models.RouterInterfaceMetric{}).Count(&samples)
	assert.Zero(t, samples)

	// The reset reading becomes the new baseline, so the next minute has a rate.
	f.counters[0].InOctets, f.counters[0].OutOctets = 500+7_500, 500+7_500
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(2*time.Minute)))
	f.db.Model(&models.RouterInterfaceMetric{}).Count(&samples)
	assert.EqualValues(t, 1, samples)
}

func TestPollInterfacesSkipsDisabledAndUnknownInterfaces(t *testing.T) {
	f := newPollFixture(t)
	off := f.addInterface(t, "vlan-old", "", true)
	start := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

	f.counters = []connectivity.IfCounters{{Name: "vlan-old", InOctets: 1}, {Name: "<pppoe-x>", InOctets: 1}}
	require.NoError(t, f.service.PollInterfaces(f.router, start))

	var stored models.RouterInterface
	require.NoError(t, f.db.First(&stored, "id = ?", off.ID).Error)
	assert.Nil(t, stored.PrevSampleAt)
}

func TestPollInterfacesMarksTheRouterDownWhenSNMPFails(t *testing.T) {
	f := newPollFixture(t)
	f.snmpErr = errors.New("request timeout")

	err := f.service.PollInterfaces(f.router, time.Now())

	assert.Error(t, err)
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterStatusDown, router.Status)
}

func (f *pollFixture) storedRouter(t *testing.T) models.MikrotikRouter {
	t.Helper()
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	return router
}

func TestPollInterfacesStoresHealth(t *testing.T) {
	f := newPollFixture(t)
	now := time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)
	f.gauges = []connectivity.Gauge{
		{Name: "cpu-temperature", Value: 43, Unit: 1},
		{Name: "psu1-state", Value: 0, Unit: 6},
		{Name: "psu2-voltage", Value: 121, Unit: 3},
	}

	require.NoError(t, f.service.PollInterfaces(f.router, now))

	router := f.storedRouter(t)
	assert.JSONEq(t, `[{"name":"cpu-temperature","value":43,"unit":1},`+
		`{"name":"psu1-state","value":0,"unit":6},{"name":"psu2-voltage","value":121,"unit":3}]`, string(router.Health))
	require.NotNil(t, router.HealthAt)
	assert.True(t, router.HealthAt.Equal(now))
}

func TestPollInterfacesKeepsPollingWhenHealthFails(t *testing.T) {
	f := newPollFixture(t)
	first := time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)
	f.gauges = []connectivity.Gauge{{Name: "psu1-state", Value: 0, Unit: 6}}
	require.NoError(t, f.service.PollInterfaces(f.router, first))

	f.gaugeErr = errors.New("no such name")
	require.NoError(t, f.service.PollInterfaces(f.router, first.Add(time.Minute)))

	router := f.storedRouter(t)
	assert.Equal(t, models.RouterStatusUp, router.Status)
	assert.JSONEq(t, `[{"name":"psu1-state","value":0,"unit":6}]`, string(router.Health))
	require.NotNil(t, router.HealthAt)
	assert.True(t, router.HealthAt.Equal(first), "a failed read must not make the old reading look fresh")
}

func TestPollInterfacesIgnoresEmptyHealth(t *testing.T) {
	f := newPollFixture(t)
	first := time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)
	f.gauges = []connectivity.Gauge{{Name: "fan1-speed", Value: 5654, Unit: 2}}
	require.NoError(t, f.service.PollInterfaces(f.router, first))

	f.gauges = nil
	require.NoError(t, f.service.PollInterfaces(f.router, first.Add(time.Minute)))

	router := f.storedRouter(t)
	assert.JSONEq(t, `[{"name":"fan1-speed","value":5654,"unit":2}]`, string(router.Health))
	require.NotNil(t, router.HealthAt)
	assert.True(t, router.HealthAt.Equal(first))
}

func TestPollBGPStoresSessionsMatchedToTheirVLANAndTheRoutersResources(t *testing.T) {
	f := newPollFixture(t)
	rs := f.addInterface(t, "vlan-rs", "198.18.0.140/23", false)
	f.addInterface(t, "vlan-transit", "198.51.100.1/31", false)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	f.api.resource = mikrotik.SystemResource{Uptime: "4w6d1h44m21s", Version: "7.24.2 (stable)", CPULoad: "21",
		FreeMemory: "12669456384", TotalMemory: "17179869184", BoardName: "CCR2116-12G-4S+"}

	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	var sessions []models.RouterBGPSession
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Order("name").Find(&sessions).Error)
	require.Len(t, sessions, 5)
	for _, s := range sessions {
		if s.Name == "peer-rs2" {
			require.NotNil(t, s.InterfaceID)
			assert.Equal(t, rs.ID, *s.InterfaceID)
		}
	}

	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusOK, router.APIStatus)
	require.NotNil(t, router.CPULoad)
	assert.EqualValues(t, 21, *router.CPULoad)
	require.NotNil(t, router.UptimeSeconds)
	assert.EqualValues(t, 34*86400+3600+44*60+21, *router.UptimeSeconds)
}

func TestPollBGPWritesEventsAtTheRoutersLocalTime(t *testing.T) {
	f := newPollFixture(t)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	f.api.sessions[0].Established, f.api.sessions[0].LastStopped = "", "2026-10-12 08:00:05"
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	var events []models.RouterBGPEvent
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "peer-transit", events[0].SessionName)
	assert.Equal(t, time.Date(2026, 10, 12, 1, 0, 5, 0, time.UTC), events[0].OccurredAt.UTC())
}

func TestPollBGPDropsSessionsNoLongerConfigured(t *testing.T) {
	f := newPollFixture(t)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	f.api.connections = f.api.connections[:1]
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	var count int64
	f.db.Model(&models.RouterBGPSession{}).Where("router_id = ?", f.router.ID).Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestPollBGPRecordsAChangedCertificateWithWhatTheRouterNowPresents(t *testing.T) {
	f := newPollFixture(t)
	f.api.err = mikrotik.ErrCertificateChanged

	err := f.service.PollBGP(f.router, time.Now())

	assert.True(t, errors.Is(err, mikrotik.ErrCertificateChanged))
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusCertificateChanged, router.APIStatus)
	require.NotNil(t, router.PresentedFingerprint)
	assert.Equal(t, "ef"+testFingerprint[2:], *router.PresentedFingerprint)
}

func TestPollBGPRecordsOtherAPIErrors(t *testing.T) {
	f := newPollFixture(t)
	f.api.err = mikrotik.ErrUnauthorized

	assert.Error(t, f.service.PollBGP(f.router, time.Now()))

	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusError, router.APIStatus)
	require.NotNil(t, router.APIError)
	assert.Contains(t, *router.APIError, "rejected")
}

func TestPollBGPRejectsAnUnreadableClockOffset(t *testing.T) {
	f := newPollFixture(t)
	f.api.clock.GMTOffset = "WIB"
	f.api.connections = borderConnections()

	assert.Error(t, f.service.PollBGP(f.router, time.Now()), "guessing UTC would shift every event by hours")
}

func TestPollBGPClosesTheConnection(t *testing.T) {
	f := newPollFixture(t)

	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	assert.Equal(t, 1, f.api.closed)
}

// A refused login or a dropped session is an API error on the router row, not
// a crash, and the next poll opens a fresh connection.
func TestPollBGPRecordsAFailedOpen(t *testing.T) {
	f := newPollFixture(t)
	f.openErr = mikrotik.ErrUnauthorized

	err := f.service.PollBGP(f.router, time.Now())

	require.ErrorIs(t, err, mikrotik.ErrUnauthorized)
	var stored models.MikrotikRouter
	require.NoError(t, f.db.First(&stored, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusError, stored.APIStatus)
	require.NotNil(t, stored.APIError)
	assert.Contains(t, *stored.APIError, "rejected")
}

// A router switched from api to a TLS protocol has no pin, so the open itself
// reports the certificate and the admin must be able to accept it.
func TestPollBGPRecordsAChangedCertificateRaisedByTheOpen(t *testing.T) {
	f := newPollFixture(t)
	f.openErr = mikrotik.ErrCertificateChanged

	err := f.service.PollBGP(f.router, time.Now())

	require.ErrorIs(t, err, mikrotik.ErrCertificateChanged)
	var stored models.MikrotikRouter
	require.NoError(t, f.db.First(&stored, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusCertificateChanged, stored.APIStatus)
	require.NotNil(t, stored.PresentedFingerprint)
}
