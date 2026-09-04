package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSMediaRetention removes attachments once they are old enough that nobody
// opens them, which is what keeps the VPS disk from filling up at a few
// gigabytes a month.
type CSMediaRetention struct {
	db       *gorm.DB
	root     string
	keepDays int
}

// NewCSMediaRetention constructs a CSMediaRetention.
func NewCSMediaRetention(db *gorm.DB, root string, keepDays int) *CSMediaRetention {
	return &CSMediaRetention{db: db, root: root, keepDays: keepDays}
}

// attachment is one row's file, reduced to what a sweep needs. The two tables
// that write into the media root differ only in which model the path is
// forgotten on.
type attachment struct {
	id    uuid.UUID
	path  string
	table any
}

// Sweep drops attachments past the retention window and forgets their paths,
// answering how many rows it cleared and what went wrong on the ones it could
// not. That is rows, not bytes.
//
// The rows stay: a CS reading old history should still see that the customer
// sent a photo, even when the photo itself is gone. MediaMime and
// MediaFilename stay with them, so the history can still name what was sent.
func (r *CSMediaRetention) Sweep() (int, error) {
	expired, err := r.expired(time.Now().AddDate(0, 0, -r.keepDays))
	if err != nil {
		return 0, err
	}

	cleared := 0
	var failures []error
	for _, file := range expired {
		// One path that will not come off the disk used to end the whole
		// sweep, leaving every attachment behind it there until tomorrow.
		if err := r.clear(file); err != nil {
			failures = append(failures, err)
			continue
		}
		cleared++
	}
	return cleared, errors.Join(failures...)
}

// expired lists what is past the window in both tables that store an
// attachment: the chat messages and the channel updates, which go through the
// same upload into the same root and would otherwise grow there without bound.
//
// A channel update is aged by created_at rather than by a wa_timestamp. It has
// none: a channel sends no receipts, so nothing ever comes back to date a post
// by.
func (r *CSMediaRetention) expired(cutoff time.Time) ([]attachment, error) {
	var messages []models.CSMessage
	err := r.db.Where("media_path <> '' AND wa_timestamp < ?", cutoff).Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("list expired media: %w", err)
	}
	var posts []models.WAChannelPost
	err = r.db.Where("media_path <> '' AND created_at < ?", cutoff).Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("list expired channel media: %w", err)
	}

	files := make([]attachment, 0, len(messages)+len(posts))
	for _, row := range messages {
		files = append(files, attachment{id: row.ID, path: row.MediaPath, table: &models.CSMessage{}})
	}
	for _, row := range posts {
		files = append(files, attachment{id: row.ID, path: row.MediaPath, table: &models.WAChannelPost{}})
	}
	return files, nil
}

// clear removes one attachment and forgets where it was. A file somebody
// already deleted by hand still counts as cleared: the row was pointing at it
// and now is not.
func (r *CSMediaRetention) clear(file attachment) error {
	if err := os.Remove(filepath.Join(r.root, file.path)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", file.path, err)
	}
	err := r.db.Model(file.table).Where("id = ?", file.id).
		Updates(map[string]any{"media_path": "", "media_size": 0}).Error
	if err != nil {
		return fmt.Errorf("forget media path %s: %w", file.path, err)
	}
	return nil
}
