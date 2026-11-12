package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

// fakePorts is LibreNMS answering one device's port list, with each port's
// oper and admin status as the test sets them.
type fakePorts struct {
	mu    sync.Mutex
	ports map[string][2]string
	// failures is how many of the next reads answer 502.
	failures int
}

func (f *fakePorts) set(ifName, oper, admin string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ports[ifName] = [2]string{oper, admin}
}

func (f *fakePorts) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failures > 0 {
		f.failures--
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	ports := []map[string]any{}
	for name, status := range f.ports {
		ports = append(ports, map[string]any{"port_id": 1, "ifName": name, "ifOperStatus": status[0], "ifAdminStatus": status[1]})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "ports": ports})
}

type portAlertFixture struct {
	graphReportFixture
	alerts *PortAlertService
	ports  *fakePorts
	noc    *models.WAGroup
}

func newPortAlertFixture(t *testing.T) portAlertFixture {
	t.Helper()
	f := newGraphReportFixture(t)
	ports := &fakePorts{ports: map[string][2]string{}}
	server := httptest.NewServer(ports)
	t.Cleanup(server.Close)
	require.NoError(t, f.sender.settings.Set(models.SettingLibreNMSURL, server.URL, uuid.Nil))
	noc, err := NewWAGroupService(f.db).Upsert(f.group.WAAccountID, groupSnap("8@g.us", "NOC", "62812"))
	require.NoError(t, err)
	_, err = NewPortAlertDestinationService(f.db).Set(PortAlertDestinationInput{WAGroupID: &noc.ID, Enabled: true})
	require.NoError(t, err)
	return portAlertFixture{
		graphReportFixture: f, alerts: NewPortAlertService(f.db, f.sender, zap.NewNop()), ports: ports, noc: noc,
	}
}

// report registers a report on group whose ports are named by labels, each
// on ifName "if-<label>" of device 7, "core", and each opted in to notices
// when notify is set.
func (f portAlertFixture) report(t *testing.T, groupID uuid.UUID, customer string, notify bool, labels ...string) {
	t.Helper()
	in := reportInput(groupID, labels...)
	in.CustomerName = customer
	if notify {
		notifyEveryPort(&in)
	}
	_, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
}

func notifyEveryPort(in *GraphReportInput) {
	for i := range in.Ports {
		in.Ports[i].NotifyStatus = true
	}
}

func (f portAlertFixture) processAt(t *testing.T, hour, minute int) {
	t.Helper()
	f.alerts.now = func() time.Time { return atWIB(12, hour, minute) }
	require.NoError(t, f.alerts.Process(context.Background(), 7))
}

// texts answers what was queued into a group, oldest first.
func (f portAlertFixture) texts(t *testing.T, groupID uuid.UUID) []string {
	t.Helper()
	var bodies []string
	require.NoError(t, f.db.Model(&models.WAGroupMessage{}).Where("group_id = ?", groupID).
		Order("created_at ASC").Pluck("body", &bodies).Error)
	return bodies
}

func (f portAlertFixture) otherGroup(t *testing.T, jid string) *models.WAGroup {
	t.Helper()
	group, err := NewWAGroupService(f.db).Upsert(f.group.WAAccountID, groupSnap(jid, "PT Lain", "62813"))
	require.NoError(t, err)
	return group
}

func TestPortDownNotifiesTheReportAndTheNOCOnce(t *testing.T) {
	f := newPortAlertFixture(t)
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink")
	f.ports.set("if-Uplink", "down", "up")

	f.processAt(t, 8, 15)
	f.processAt(t, 8, 20)

	assert.Equal(t, []string{"🔴 DOWN — PT Contoh\nPort: Uplink (core if-Uplink)\nSejak: 08:15 WIB"}, f.texts(t, f.group.ID),
		"the alert repeated finds nothing new")
	assert.Equal(t, []string{"🔴 DOWN\nPT Contoh — Uplink (core if-Uplink)\nSejak: 08:15 WIB"}, f.texts(t, f.noc.ID))
	assert.Len(t, f.notifier.queued, 2, "the wa process is woken for each")
}

// LibreNMS alerts once per state change, so a read that fails must be tried
// again or the outage, and later its recovery, go unannounced.
func TestPortDownIsStillNoticedWhenTheFirstReadFails(t *testing.T) {
	f := newPortAlertFixture(t)
	delays := portReadRetryDelays
	portReadRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { portReadRetryDelays = delays })
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink")
	f.ports.set("if-Uplink", "down", "up")
	f.ports.failures = 2

	f.processAt(t, 8, 15)

	assert.Equal(t, []string{"🔴 DOWN — PT Contoh\nPort: Uplink (core if-Uplink)\nSejak: 08:15 WIB"}, f.texts(t, f.group.ID))
}

func TestPortReadGivesUpAfterItsRetries(t *testing.T) {
	f := newPortAlertFixture(t)
	delays := portReadRetryDelays
	portReadRetryDelays = []time.Duration{time.Millisecond}
	t.Cleanup(func() { portReadRetryDelays = delays })
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink")
	f.ports.set("if-Uplink", "down", "up")
	f.ports.failures = 2

	require.Error(t, f.alerts.Process(context.Background(), 7))

	assert.Empty(t, f.texts(t, f.group.ID))
	assert.Zero(t, f.ports.failures, "one attempt and one retry")
}

func TestPortRecoveryCarriesHowLongItWasDown(t *testing.T) {
	f := newPortAlertFixture(t)
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink")
	f.ports.set("if-Uplink", "lowerLayerDown", "up")
	f.processAt(t, 8, 15)

	f.ports.set("if-Uplink", "up", "up")
	f.processAt(t, 9, 20)

	texts := f.texts(t, f.group.ID)
	require.Len(t, texts, 2)
	assert.Equal(t, "🟢 UP — PT Contoh\nPort: Uplink (core if-Uplink)\nDown selama 1 jam 5 menit", texts[1])
	assert.Equal(t, "🟢 UP\nPT Contoh — Uplink (core if-Uplink), down selama 1 jam 5 menit", f.texts(t, f.noc.ID)[1])
}

func TestPortShutDownByAnAdminIsNoOutage(t *testing.T) {
	f := newPortAlertFixture(t)
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink")
	f.ports.set("if-Uplink", "down", "down")

	f.processAt(t, 8, 15)

	assert.Empty(t, f.texts(t, f.group.ID))
	assert.Empty(t, f.texts(t, f.noc.ID))
	var rows int64
	require.NoError(t, f.db.Model(&models.PortLinkState{}).Count(&rows).Error)
	assert.Zero(t, rows, "nothing is recorded either")
}

func TestPortFirstSeenUpIsSilentAndItsFallIsNot(t *testing.T) {
	f := newPortAlertFixture(t)
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink")
	f.ports.set("if-Uplink", "up", "up")
	f.processAt(t, 8, 0)
	assert.Empty(t, f.texts(t, f.group.ID), "a port found up has nothing to recover from")

	f.ports.set("if-Uplink", "down", "up")
	f.processAt(t, 8, 15)

	assert.Len(t, f.texts(t, f.group.ID), 1)
}

func TestPortNoticesReachOnlyEnabledReportsThatAskForThem(t *testing.T) {
	f := newPortAlertFixture(t)
	f.report(t, f.group.ID, "PT Contoh", false, "Uplink")
	off := f.otherGroup(t, "7@g.us")
	in := reportInput(off.ID, "Uplink")
	in.CustomerName, in.Enabled = "PT Lain", false
	notifyEveryPort(&in)
	_, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	f.ports.set("if-Uplink", "down", "up")

	f.processAt(t, 8, 15)

	assert.Empty(t, f.texts(t, f.group.ID), "not opted in")
	assert.Empty(t, f.texts(t, off.ID), "report turned off")
	assert.Equal(t, []string{
		"🔴 DOWN\nPT Contoh — Uplink (core if-Uplink)\nPT Lain — Uplink (core if-Uplink)\nSejak: 08:15 WIB",
	}, f.texts(t, f.noc.ID), "the NOC hears of every monitored port")
}

func TestCustomerNoticeNamesOnlyThePortsOptedIn(t *testing.T) {
	f := newPortAlertFixture(t)
	in := reportInput(f.group.ID, "Uplink", "Backup")
	in.Ports[1].NotifyStatus = true
	_, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	f.ports.set("if-Uplink", "down", "up")
	f.ports.set("if-Backup", "down", "up")

	f.processAt(t, 8, 15)

	assert.Equal(t, []string{"🔴 DOWN — PT Contoh\nPort: Backup (core if-Backup)\nSejak: 08:15 WIB"}, f.texts(t, f.group.ID))
	assert.Equal(t, []string{
		"🔴 DOWN\nPT Contoh — Uplink (core if-Uplink)\nPT Contoh — Backup (core if-Backup)\nSejak: 08:15 WIB",
	}, f.texts(t, f.noc.ID), "the NOC hears of both")
}

func TestReportWithNoOptedInPortChangedTellsOnlyTheNOC(t *testing.T) {
	f := newPortAlertFixture(t)
	in := reportInput(f.group.ID, "Uplink", "Backup")
	in.Ports[0].NotifyStatus = true
	_, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	f.ports.set("if-Uplink", "up", "up")
	f.ports.set("if-Backup", "down", "up")

	f.processAt(t, 8, 15)

	assert.Empty(t, f.texts(t, f.group.ID), "the port that fell is not opted in")
	assert.Equal(t, []string{"🔴 DOWN\nPT Contoh — Backup (core if-Backup)\nSejak: 08:15 WIB"}, f.texts(t, f.noc.ID))
}

func TestPortSharedByTwoReportsNotifiesBoth(t *testing.T) {
	f := newPortAlertFixture(t)
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink")
	other := f.otherGroup(t, "7@g.us")
	f.report(t, other.ID, "PT Lain", true, "Uplink")
	f.ports.set("if-Uplink", "down", "up")

	f.processAt(t, 8, 15)

	assert.Equal(t, []string{"🔴 DOWN — PT Contoh\nPort: Uplink (core if-Uplink)\nSejak: 08:15 WIB"}, f.texts(t, f.group.ID))
	assert.Equal(t, []string{"🔴 DOWN — PT Lain\nPort: Uplink (core if-Uplink)\nSejak: 08:15 WIB"}, f.texts(t, other.ID))
	assert.Len(t, f.texts(t, f.noc.ID), 1, "one NOC message lists both")
}

func TestPortsOfOneReportChangingTogetherGoInOneMessage(t *testing.T) {
	f := newPortAlertFixture(t)
	f.report(t, f.group.ID, "PT Contoh", true, "Uplink", "Backup", "Spare")
	f.ports.set("if-Uplink", "down", "up")
	f.ports.set("if-Backup", "down", "up")
	f.ports.set("if-Spare", "up", "up")

	f.processAt(t, 8, 15)

	assert.Equal(t, []string{
		"🔴 DOWN — PT Contoh\nPort: Uplink (core if-Uplink)\nPort: Backup (core if-Backup)\nSejak: 08:15 WIB",
	}, f.texts(t, f.group.ID), "in the report's order, the port still up left out")
}

func TestPortNoticeGoesToTheReportsTelegramChat(t *testing.T) {
	f := newPortAlertFixture(t)
	fake, chat := f.useTelegram(t)
	in := reportInput(uuid.Nil, "Uplink")
	in.TelegramChatID = &chat.ID
	notifyEveryPort(&in)
	_, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	f.ports.set("if-Uplink", "down", "up")

	f.processAt(t, 8, 15)

	posts := fake.sent()
	require.Len(t, posts, 1)
	assert.Equal(t, "sendMessage", posts[0].method)
	assert.Equal(t, chat.ChatID, posts[0].chatID)
	assert.Equal(t, "🔴 DOWN — PT Contoh\nPort: Uplink (core if-Uplink)\nSejak: 08:15 WIB", posts[0].text)
}

// A Telegram chat the bot was removed from is marked as such, and the
// WhatsApp group still gets its notice.
func TestPortNoticeRefusedByTelegramDeactivatesTheChat(t *testing.T) {
	f := newPortAlertFixture(t)
	fake, chat := f.useTelegram(t)
	fake.refuse(http.StatusForbidden, `{"ok":false,"description":"Forbidden: bot was kicked"}`)
	in := reportInput(f.group.ID, "Uplink")
	in.TelegramChatID = &chat.ID
	notifyEveryPort(&in)
	_, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	f.ports.set("if-Uplink", "down", "up")

	f.processAt(t, 8, 15)

	assert.Len(t, f.texts(t, f.group.ID), 1)
	var stored models.TelegramChat
	require.NoError(t, f.db.First(&stored, "id = ?", chat.ID).Error)
	assert.False(t, stored.Active)
}

func TestOutageDurationReadsInIndonesian(t *testing.T) {
	start := atWIB(12, 8, 0)
	for _, tc := range []struct {
		down time.Duration
		want string
	}{
		{40 * time.Second, "< 1 menit"},
		{12 * time.Minute, "12 menit"},
		{65 * time.Minute, "1 jam 5 menit"},
		{2 * time.Hour, "2 jam"},
		{27*time.Hour + 3*time.Minute, "1 hari 3 jam 3 menit"},
	} {
		got := outageDuration(portTransition{since: start, at: start.Add(tc.down)})
		assert.Equal(t, tc.want, got, tc.down.String())
	}
}

func TestWebhookTokenMustMatchAConfiguredLongEnoughSecret(t *testing.T) {
	f := newPortAlertFixture(t)
	assert.False(t, f.alerts.WebhookAuthorized(""), "nothing configured")

	require.NoError(t, f.sender.settings.Set(models.SettingLibreNMSWebhookToken, "short", uuid.Nil))
	assert.False(t, f.alerts.WebhookAuthorized("short"), "a short secret is treated as unset")

	require.NoError(t, f.sender.settings.Set(models.SettingLibreNMSWebhookToken, "0123456789abcdef", uuid.Nil))
	assert.False(t, f.alerts.WebhookAuthorized("0123456789abcdeX"))
	assert.True(t, f.alerts.WebhookAuthorized("0123456789abcdef"))
}
