package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// secondNumber pairs another TikMan number into the fixture's WhatsApp group,
// mirrored as a wa_groups row of its own with the same JID.
func (f graphReportFixture) secondNumber(t *testing.T, label, jid string) (models.WAAccount, *models.WAGroup) {
	t.Helper()
	account := models.WAAccount{Label: label, JID: jid, Status: models.WAAccountConnected}
	require.NoError(t, f.db.Create(&account).Error)
	twin, err := NewWAGroupService(f.db).Upsert(account.ID, groupSnap(f.group.JID, f.group.Name, "62811"))
	require.NoError(t, err)
	return account, twin
}

func TestGraphReportRefusesTheSameGroupThroughAnotherNumber(t *testing.T) {
	f := newGraphReportFixture(t)
	f.createReport(t, "Uplink")
	_, twin := f.secondNumber(t, "Kantor", "62822@s.whatsapp.net")

	_, err := f.reports.Create(reportInput(twin.ID, "Lain"), uuid.New())

	assert.True(t, errors.Is(err, ErrGraphReportGroupTaken))
	assert.EqualError(t, err, "Grup ini sudah punya laporan lewat nomor CS Utama")
	var count int64
	require.NoError(t, f.db.Model(&models.GraphReport{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestGraphReportTakenNamesAnUnlabelledNumberByItsNumber(t *testing.T) {
	f := newGraphReportFixture(t)
	_, twin := f.secondNumber(t, "", "62822:7@s.whatsapp.net")
	_, err := f.reports.Create(reportInput(twin.ID, "Uplink"), uuid.New())
	require.NoError(t, err)

	_, err = f.reports.Create(reportInput(f.group.ID, "Lain"), uuid.New())

	assert.EqualError(t, err, "Grup ini sudah punya laporan lewat nomor 62822")
}

func TestGraphReportUpdateRefusesMovingOntoAGroupReportedThroughAnotherNumber(t *testing.T) {
	f := newGraphReportFixture(t)
	f.createReport(t, "Uplink")
	account, twin := f.secondNumber(t, "Kantor", "62822@s.whatsapp.net")
	other, err := NewWAGroupService(f.db).Upsert(account.ID, groupSnap("10@g.us", "LAIN", "62833"))
	require.NoError(t, err)
	report, err := f.reports.Create(reportInput(other.ID, "Lain"), uuid.New())
	require.NoError(t, err)

	_, err = f.reports.Update(report.ID, reportInput(twin.ID, "Lain"))

	assert.True(t, errors.Is(err, ErrGraphReportGroupTaken))
	assert.Equal(t, &other.ID, f.load(t, report.ID).WAGroupID, "the report stays where it was")
}

func TestGraphReportMayMoveToAnotherNumberInTheSameGroup(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	account, twin := f.secondNumber(t, "Kantor", "62822@s.whatsapp.net")

	same, err := f.reports.Update(report.ID, reportInput(f.group.ID, "Uplink"))
	require.NoError(t, err, "a report updated in place is not its own rival")
	assert.Equal(t, "CS Utama", same.WAAccountLabel)

	moved, err := f.reports.Update(report.ID, reportInput(twin.ID, "Uplink"))
	require.NoError(t, err)
	assert.Equal(t, &account.ID, moved.WAAccountID)
	assert.Equal(t, "Kantor", moved.WAAccountLabel)
}

func TestGraphReportListNamesEachReportsNumber(t *testing.T) {
	f := newGraphReportFixture(t)
	f.createReport(t, "Uplink")
	account, _ := f.secondNumber(t, "", "62822@s.whatsapp.net")
	other, err := NewWAGroupService(f.db).Upsert(account.ID, groupSnap("10@g.us", "LAIN", "62833"))
	require.NoError(t, err)
	in := reportInput(other.ID, "Lain")
	in.CustomerName = "PT Lain"
	_, err = f.reports.Create(in, uuid.New())
	require.NoError(t, err)

	views, err := f.reports.List()
	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, &f.group.WAAccountID, views[0].WAAccountID)
	assert.Equal(t, "CS Utama", views[0].WAAccountLabel)
	assert.Equal(t, &account.ID, views[1].WAAccountID)
	assert.Equal(t, "62822", views[1].WAAccountLabel)
}

func TestSendNowLeavesEveryTikManNumberUntagged(t *testing.T) {
	f := newGraphReportFixture(t)
	f.addMembers(t, 1)
	f.secondNumber(t, "Kantor", "62822:3@s.whatsapp.net")
	require.NoError(t, f.db.Create(&[]models.WAGroupParticipant{
		{GroupID: f.group.ID, JID: "62822@s.whatsapp.net", Phone: "62822", Role: models.GroupRoleMember},
		{GroupID: f.group.ID, JID: "5555@lid", LID: "5555@lid", Phone: "62822", Role: models.GroupRoleMember},
	}).Error)
	report := f.mentionAllReport(t, true)

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	msg := f.queuedMessage(t)
	assert.ElementsMatch(t, []string{"62811@s.whatsapp.net", "1000@lid"}, Mentions(&msg))
}

func TestSendNowLeavesAnotherTikManNumberKnownOnlyByItsLIDUntagged(t *testing.T) {
	f := newGraphReportFixture(t)
	f.addMembers(t, 1)
	_, twin := f.secondNumber(t, "Kantor", "62822@s.whatsapp.net")
	require.NoError(t, f.db.Create(&[]models.WAGroupParticipant{
		{GroupID: f.group.ID, JID: "7777@lid", LID: "7777@lid", Role: models.GroupRoleMember},
		{GroupID: twin.ID, JID: "7777@lid", LID: "7777@lid", Role: models.GroupRoleMember, IsSelf: true},
	}).Error)
	report := f.mentionAllReport(t, true)

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	msg := f.queuedMessage(t)
	assert.ElementsMatch(t, []string{"62811@s.whatsapp.net", "1000@lid"}, Mentions(&msg))
}
