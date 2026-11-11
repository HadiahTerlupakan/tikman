package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

func TestNOCDestinationUnsetReadsAsNoneEnabled(t *testing.T) {
	f := newGraphReportFixture(t)

	view, err := NewPortAlertDestinationService(f.db).Get()

	require.NoError(t, err)
	assert.Equal(t, PortAlertDestinationView{Enabled: true}, *view)
}

// The NOC may read its notices in a group that also carries a customer's
// report; the one-report-per-group rule is the reports' alone.
func TestNOCDestinationMayShareAReportsGroup(t *testing.T) {
	f := newGraphReportFixture(t)
	f.createReport(t, "Uplink")
	destinations := NewPortAlertDestinationService(f.db)

	view, err := destinations.Set(PortAlertDestinationInput{WAGroupID: &f.group.ID, Enabled: false})

	require.NoError(t, err)
	assert.Equal(t, &f.group.ID, view.WAGroupID)
	assert.Equal(t, "TEST MRTG", view.GroupName)
	assert.Equal(t, &f.group.WAAccountID, view.WAAccountID)
	assert.False(t, view.Enabled, "a false enabled is stored, not the column's default")

	view, err = destinations.Set(PortAlertDestinationInput{Enabled: true})
	require.NoError(t, err)
	assert.Nil(t, view.WAGroupID, "both destinations may be cleared")
	var rows int64
	require.NoError(t, f.db.Model(&models.PortAlertDestination{}).Count(&rows).Error)
	assert.Equal(t, int64(1), rows, "the one row is rewritten")
}

func TestNOCDestinationRefusesWhatAReportWould(t *testing.T) {
	f := newGraphReportFixture(t)
	destinations := NewPortAlertDestinationService(f.db)
	missing := uuid.New()
	_, err := destinations.Set(PortAlertDestinationInput{WAGroupID: &missing})
	assert.True(t, errors.Is(err, ErrValidation))

	gone := models.TelegramChat{ChatID: -100777, Title: "Lama", Type: telegram.ChatTypeSupergroup}
	require.NoError(t, f.db.Create(&gone).Error)
	require.NoError(t, f.db.Model(&gone).Update("active", false).Error)
	_, err = destinations.Set(PortAlertDestinationInput{TelegramChatID: &gone.ID})
	assert.ErrorContains(t, err, "bot sudah tidak ada")

	thread := int64(9)
	_, err = destinations.Set(PortAlertDestinationInput{TelegramThreadID: &thread})
	assert.ErrorContains(t, err, "topik Telegram dipilih tanpa grup Telegram")
}

func TestDeletingTheNOCGroupLeavesTheDestinationWithoutIt(t *testing.T) {
	f := newGraphReportFixture(t)
	destinations := NewPortAlertDestinationService(f.db)
	_, err := destinations.Set(PortAlertDestinationInput{WAGroupID: &f.group.ID, Enabled: true})
	require.NoError(t, err)
	require.NoError(t, f.db.Model(f.group).Update("has_left", true).Error)

	require.NoError(t, NewWAGroupService(f.db).DeleteLeft(f.group.ID, f.mediaRoot))

	view, err := destinations.Get()
	require.NoError(t, err)
	assert.Nil(t, view.WAGroupID)
	assert.True(t, view.Enabled)
}

// A supergroup row the NOC posts to must survive the group's upgrade as a
// report's does; deleting it would silently unset the NOC's chat.
func TestTelegramChatMigrationKeepsTheNOCsSupergroupRow(t *testing.T) {
	f := newGraphReportFixture(t)
	chats := NewTelegramChatService(f.db)
	require.NoError(t, chats.Apply(memberUpdate(1, telegram.Chat{ID: -5, Title: "NOC", Type: telegram.ChatTypeGroup}, telegram.StatusMember)))
	require.NoError(t, chats.Apply(memberUpdate(2, telegram.Chat{ID: -1005, Title: "NOC", Type: telegram.ChatTypeSupergroup}, telegram.StatusMember)))
	super := chatRows(t, f.db)[0]
	_, err := NewPortAlertDestinationService(f.db).Set(PortAlertDestinationInput{TelegramChatID: &super.ID, Enabled: true})
	require.NoError(t, err)

	require.NoError(t, chats.Migrate(-5, -1005))

	rows := chatRows(t, f.db)
	require.Len(t, rows, 2)
	assert.Equal(t, super.ID, rows[0].ID)
	assert.Equal(t, int64(-1005), rows[0].ChatID)
}

func TestReportStoresItsPortNoticeChoice(t *testing.T) {
	f := newGraphReportFixture(t)
	in := reportInput(f.group.ID, "Uplink")
	in.NotifyPortStatus = true
	created, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	assert.True(t, created.NotifyPortStatus)

	in.NotifyPortStatus = false
	updated, err := f.reports.Update(created.ID, in)
	require.NoError(t, err)
	assert.False(t, updated.NotifyPortStatus)
}
