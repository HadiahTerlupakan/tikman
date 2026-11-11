package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// addMembers puts n more members into the fixture's group, plus our own
// number, which a tag of everyone must leave out.
func (f graphReportFixture) addMembers(t *testing.T, n int) {
	t.Helper()
	rows := []models.WAGroupParticipant{{
		GroupID: f.group.ID, JID: "62899@s.whatsapp.net", Phone: "62899", Role: models.GroupRoleMember, IsSelf: true,
	}}
	for i := 0; i < n; i++ {
		rows = append(rows, models.WAGroupParticipant{
			GroupID: f.group.ID, JID: fmt.Sprintf("%d@lid", 1000+i), Role: models.GroupRoleMember,
		})
	}
	require.NoError(t, f.db.Create(&rows).Error)
}

func (f graphReportFixture) mentionAllReport(t *testing.T, on bool) *GraphReportView {
	t.Helper()
	in := reportInput(f.group.ID, "Uplink")
	in.MentionAll = on
	report, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	return report
}

func (f graphReportFixture) queuedMessage(t *testing.T) models.WAGroupMessage {
	t.Helper()
	var msg models.WAGroupMessage
	require.NoError(t, f.db.Where("group_id = ?", f.group.ID).First(&msg).Error)
	return msg
}

func TestSendNowTagsEveryMemberButUs(t *testing.T) {
	f := newGraphReportFixture(t)
	f.addMembers(t, 2)
	report := f.mentionAllReport(t, true)

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	msg := f.queuedMessage(t)
	assert.ElementsMatch(t, []string{"62811@s.whatsapp.net", "1000@lid", "1001@lid"}, Mentions(&msg))
	assert.Equal(t, GraphReportCaption("", "PT Contoh"), msg.Body, "the caption names nobody")
}

func TestSendNowTagsNobodyByDefault(t *testing.T) {
	f := newGraphReportFixture(t)
	f.addMembers(t, 2)
	report := f.mentionAllReport(t, false)

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	msg := f.queuedMessage(t)
	assert.Empty(t, msg.Mentions)
}

func TestSendNowFailsAGroupTooLargeToTag(t *testing.T) {
	f := newGraphReportFixture(t)
	f.addMembers(t, maxMentions) // with the fixture's own member, one past the cap
	report := f.mentionAllReport(t, true)

	err := f.sender.SendNow(context.Background(), report.ID)
	assert.True(t, errors.Is(err, ErrGraphReportTooManyMentions))
	assert.Zero(t, f.queuedCount(t))
	assert.Equal(t, "grup terlalu besar untuk tag semua (maks 1024 anggota)", f.load(t, report.ID).LastError)
}

func TestGraphReportMentionAllIsStoredAndRewritten(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	assert.False(t, report.MentionAll)
	assert.False(t, f.load(t, report.ID).MentionAll, "an existing report stays untagged")

	in := reportInput(f.group.ID, "Uplink")
	in.MentionAll = true
	updated, err := f.reports.Update(report.ID, in)
	require.NoError(t, err)
	assert.True(t, updated.MentionAll)

	in.MentionAll = false
	updated, err = f.reports.Update(report.ID, in)
	require.NoError(t, err)
	assert.False(t, updated.MentionAll)
}
