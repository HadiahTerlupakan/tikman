package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestSendNowQueuesOneImagePerPortInOrder(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink", "Backup", "IX")
	f.sender.now = func() time.Time { return atWIB(11, 6, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	var msgs []models.WAGroupMessage
	require.NoError(t, f.db.Where("group_id = ?", f.group.ID).Order("created_at ASC").Find(&msgs).Error)
	require.Len(t, msgs, 3)
	assert.Equal(t, "📊 Laporan Trafik Harian — PT Contoh\n10 Nov 06:00 – 11 Nov 06:00 WIB\nPort: Uplink", msgs[0].Body)
	assert.Equal(t, "Port: Backup", msgs[1].Body)
	assert.Equal(t, "Port: IX", msgs[2].Body)
	for _, msg := range msgs {
		assert.Nil(t, msg.SenderUserID)
		assert.Equal(t, GraphReportSenderName, msg.SenderName)
		assert.Equal(t, models.MessageKindImage, msg.Kind)
		assert.Equal(t, models.MessageQueued, msg.Status)
		assert.Equal(t, "image/png", msg.MediaMime)
		assert.False(t, filepath.IsAbs(msg.MediaPath), "media path is relative to the media root")
		info, err := os.Stat(filepath.Join(f.mediaRoot, msg.MediaPath))
		require.NoError(t, err)
		assert.Equal(t, msg.MediaSize, info.Size())
	}
	assert.Len(t, f.notifier.queued, 3)

	stored := f.load(t, report.ID)
	assert.Nil(t, stored.LastSentOn, "a manual send is not the day's report")
	assert.Empty(t, stored.LastError)
	require.NotNil(t, stored.LastAttemptAt)
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
