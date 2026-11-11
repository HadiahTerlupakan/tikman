package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestSendNowQueuesTwoPortsToAnImage(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink", "Backup")
	f.sender.now = func() time.Time { return atWIB(11, 6, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	msgs := f.queuedMessages(t)
	require.Len(t, msgs, 1)
	msg := msgs[0]
	assert.Equal(t, "Selamat pagi Rekan,\n\nBerikut kami lampirkan hasil monitoring trafik 24 jam terakhir "+
		"untuk PT Contoh.\n\nTerima kasih", msg.Body)
	assert.Nil(t, msg.SenderUserID)
	assert.Equal(t, GraphReportSenderName, msg.SenderName)
	assert.Equal(t, models.MessageKindImage, msg.Kind)
	assert.Equal(t, models.MessageQueued, msg.Status)
	assert.Equal(t, "image/png", msg.MediaMime)
	assert.Equal(t, "PT Contoh 11-11-2026.png", msg.MediaFilename, "a single image is not numbered")
	assert.False(t, filepath.IsAbs(msg.MediaPath), "media path is relative to the media root")
	assert.Equal(t, 2, f.sectionsIn(t, msg))
	assert.Len(t, f.notifier.queued, 1)

	stored := f.load(t, report.ID)
	assert.Nil(t, stored.LastSentOn, "a manual send is not the day's report")
	assert.Empty(t, stored.LastError)
	require.NotNil(t, stored.LastAttemptAt)
}

func TestSendNowSplitsALongReportTwoPortsAnImage(t *testing.T) {
	f := newGraphReportFixture(t)
	f.addMembers(t, 1)
	in := reportInput(f.group.ID, "A", "B", "C", "D", "E")
	in.MentionAll = true
	report, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	f.sender.now = func() time.Time { return atWIB(11, 6, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	msgs := f.queuedMessages(t)
	require.Len(t, msgs, 3)
	for i, want := range []int{2, 2, 1} {
		assert.Equal(t, want, f.sectionsIn(t, msgs[i]), "image %d", i+1)
		assert.Equal(t, fmt.Sprintf("PT Contoh 11-11-2026 (%d dari 3).png", i+1), msgs[i].MediaFilename)
	}
	assert.Equal(t, GraphReportCaption("", "PT Contoh"), msgs[2].Body)
	assert.NotEmpty(t, Mentions(&msgs[2]))
	for _, rest := range msgs[:2] {
		assert.Empty(t, rest.Body, "the caption goes under the last image only")
		assert.Empty(t, Mentions(&rest), "tagging every image would notify each member once per image")
	}
	assert.Len(t, f.notifier.queued, 3)
}

func TestSendNowCaptionsWithTheReportsMessage(t *testing.T) {
	f := newGraphReportFixture(t)
	in := reportInput(f.group.ID, "Uplink")
	in.Message = "Halo {pelanggan}, ini grafik {pelanggan}."
	report, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	var msg models.WAGroupMessage
	require.NoError(t, f.db.Where("group_id = ?", f.group.ID).First(&msg).Error)
	assert.Equal(t, "Halo PT Contoh, ini grafik PT Contoh.", msg.Body)
}

func TestPreviewNeedsNoGroup(t *testing.T) {
	f := newGraphReportFixture(t)
	in := reportInput(uuid.Nil, "Uplink", "Backup")

	images, err := f.sender.Preview(context.Background(), reportInput(uuid.Nil, "A", "B", "C"))
	require.NoError(t, err)
	require.Len(t, images, 2, "the preview shows the images the group would get")
	images, err = f.sender.Preview(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, images, 1)
	_, err = png.Decode(bytes.NewReader(images[0]))
	assert.NoError(t, err)
	assert.Zero(t, f.queuedCount(t))

	in.Ports = nil
	_, err = f.sender.Preview(context.Background(), in)
	assert.True(t, errors.Is(err, ErrValidation))
}

func TestReportImageSizeIsCapped(t *testing.T) {
	assert.NoError(t, checkGraphReportImageSize(maxGraphReportImageBytes))
	assert.True(t, errors.Is(checkGraphReportImageSize(maxGraphReportImageBytes+1), ErrGraphFetch))
}

func TestSendNowQueuesNothingWhenOneGraphFails(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink", "Backup")
	f.librenms.fail("if-Backup", true)

	err := f.sender.SendNow(context.Background(), report.ID)
	assert.True(t, errors.Is(err, ErrGraphFetch))

	assert.Zero(t, f.queuedCount(t))
	assert.Empty(t, f.notifier.queued)
	stored := f.load(t, report.ID)
	assert.Contains(t, stored.LastError, "Backup")
	assert.NotNil(t, stored.LastAttemptAt)
	entries, _ := os.ReadDir(f.mediaRoot)
	assert.Empty(t, entries, "nothing written for a report that was not queued")
}

func TestSendNowRefusesAnAnnounceGroupItDoesNotAdminister(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	require.NoError(t, f.db.Model(&models.WAGroup{}).Where("id = ?", f.group.ID).
		Updates(map[string]any{"announce": true, "is_admin": false}).Error)

	err := f.sender.SendNow(context.Background(), report.ID)
	assert.True(t, errors.Is(err, ErrGroupAnnounceOnly))
	assert.Zero(t, f.queuedCount(t))
}

func TestSendNowNamesAMissingLibreNMS(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	require.NoError(t, f.sender.settings.Delete(models.SettingLibreNMSAPIToken))

	err := f.sender.SendNow(context.Background(), report.ID)
	assert.True(t, errors.Is(err, ErrLibreNMSNotConfigured))
	assert.Equal(t, "LibreNMS belum diatur di Pengaturan", f.load(t, report.ID).LastError)
}

func (f graphReportFixture) queuedMessages(t *testing.T) []models.WAGroupMessage {
	t.Helper()
	var msgs []models.WAGroupMessage
	require.NoError(t, f.db.Where("group_id = ?", f.group.ID).Order("created_at ASC").Find(&msgs).Error)
	return msgs
}

// sectionsIn counts the graphs stacked in a queued image. The fake graph is
// 2px tall, so with librenms.Compose's layout (16px padding top and bottom, a
// 56px title band per section, a 17px gap between sections) an image of n
// sections is 15+75n pixels tall.
func (f graphReportFixture) sectionsIn(t *testing.T, msg models.WAGroupMessage) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.mediaRoot, msg.MediaPath))
	require.NoError(t, err)
	assert.Equal(t, msg.MediaSize, int64(len(raw)))
	img, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	return countSections(img.Bounds().Dy())
}

func countSections(height int) int {
	return (height - 15) / 75
}
