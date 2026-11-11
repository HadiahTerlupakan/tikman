package services

import (
	"bytes"
	"context"
	"errors"
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

func TestSendNowQueuesTheWholeReportAsOneImage(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink", "Backup", "IX")
	f.sender.now = func() time.Time { return atWIB(11, 6, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	var msgs []models.WAGroupMessage
	require.NoError(t, f.db.Where("group_id = ?", f.group.ID).Find(&msgs).Error)
	require.Len(t, msgs, 1)
	msg := msgs[0]
	assert.Equal(t, "Selamat pagi Rekan,\n\nBerikut kami lampirkan hasil monitoring trafik 24 jam terakhir "+
		"untuk PT Contoh.\n\nTerima kasih", msg.Body)
	assert.Nil(t, msg.SenderUserID)
	assert.Equal(t, GraphReportSenderName, msg.SenderName)
	assert.Equal(t, models.MessageKindImage, msg.Kind)
	assert.Equal(t, models.MessageQueued, msg.Status)
	assert.Equal(t, "image/png", msg.MediaMime)
	assert.Equal(t, "PT Contoh 11-11-2026.png", msg.MediaFilename)
	assert.False(t, filepath.IsAbs(msg.MediaPath), "media path is relative to the media root")

	raw, err := os.ReadFile(filepath.Join(f.mediaRoot, msg.MediaPath))
	require.NoError(t, err)
	assert.Equal(t, msg.MediaSize, int64(len(raw)))
	img, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.Greater(t, img.Bounds().Dy(), 3*2, "three sections stacked, each taller than its 2px graph")
	assert.Len(t, f.notifier.queued, 1)

	stored := f.load(t, report.ID)
	assert.Nil(t, stored.LastSentOn, "a manual send is not the day's report")
	assert.Empty(t, stored.LastError)
	require.NotNil(t, stored.LastAttemptAt)
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

	raw, err := f.sender.Preview(context.Background(), in)
	require.NoError(t, err)
	_, err = png.Decode(bytes.NewReader(raw))
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
