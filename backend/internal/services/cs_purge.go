package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSPurgeService removes messages from the inbox along with the files they
// point at.
//
// The four things an admin can ask for — drop one message, empty a thread,
// empty a number, empty the inbox — differ only in which messages they name,
// so they share one removal rather than four copies of it. What the copies
// would drift apart on is the part that matters: the order, and putting a
// thread's summary back in step with what it still holds.
type CSPurgeService struct {
	db   *gorm.DB
	root string
}

// NewCSPurgeService constructs a CSPurgeService.
func NewCSPurgeService(db *gorm.DB, root string) *CSPurgeService {
	return &CSPurgeService{db: db, root: root}
}

// within narrows a query to the messages one purge is about.
type within func(*gorm.DB) *gorm.DB

// Message removes one message and its attachment.
func (s *CSPurgeService) Message(id uuid.UUID) (int, error) {
	return s.purge(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", id) })
}

// Conversation empties one thread. The thread itself stays in the inbox: a CS
// clearing a history has not finished with the customer.
func (s *CSPurgeService) Conversation(id uuid.UUID) (int, error) {
	return s.purge(func(db *gorm.DB) *gorm.DB { return db.Where("conversation_id = ?", id) })
}

// Account empties every thread on one number, keeping the threads themselves.
func (s *CSPurgeService) Account(id uuid.UUID) (int, error) {
	return s.purge(func(db *gorm.DB) *gorm.DB {
		return db.Where("conversation_id IN (?)",
			s.db.Model(&models.CSConversation{}).Select("id").Where("wa_account_id = ?", id))
	})
}

// Inbox empties every thread on every number.
func (s *CSPurgeService) Inbox() (int, error) {
	// GORM refuses a delete carrying no condition at all. That guard is worth
	// having everywhere else in the codebase, and this is the one caller that
	// genuinely means all of them.
	return s.purge(func(db *gorm.DB) *gorm.DB { return db.Where("1 = 1") })
}

// DeleteAccount removes a number together with every thread, message and file
// belonging to it.
func (s *CSPurgeService) DeleteAccount(id uuid.UUID) error {
	if err := s.db.First(&models.WAAccount{}, "id = ?", id).Error; err != nil {
		return err
	}

	if _, err := s.Account(id); err != nil {
		return err
	}
	if err := s.removeAvatars(id); err != nil {
		return err
	}

	// cs_conversations references wa_accounts ON DELETE RESTRICT, so the
	// threads go before the number does. The constraint is left as it is
	// rather than loosened to CASCADE: it is what makes removing a number an
	// ordered, deliberate act instead of a cascade that could one day fire
	// from somewhere nobody was thinking about.
	if err := s.db.Where("wa_account_id = ?", id).Delete(&models.CSConversation{}).Error; err != nil {
		return fmt.Errorf("delete the threads of a wa account: %w", err)
	}
	if err := s.removeChannelPosts(id); err != nil {
		return err
	}
	if err := s.db.Delete(&models.WAAccount{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete wa account: %w", err)
	}
	return nil
}

// removeChannelPosts drops one number's broadcast history and the attachments
// it points at. wa_channel_posts carries the same RESTRICT foreign key
// cs_conversations does, so without this Postgres refuses to delete any number
// that has ever posted an update.
func (s *CSPurgeService) removeChannelPosts(accountID uuid.UUID) error {
	var paths []string
	err := s.db.Model(&models.WAChannelPost{}).
		Where("wa_account_id = ? AND media_path <> ''", accountID).
		Pluck("media_path", &paths).Error
	if err != nil {
		return fmt.Errorf("find the attachments of a number's channel posts: %w", err)
	}

	// Files before rows, the order purge uses and for the same reason: the
	// other way round strands a file with its only pointer already gone.
	if err := s.removeFiles(paths); err != nil {
		return err
	}

	err = s.db.Where("wa_account_id = ?", accountID).Delete(&models.WAChannelPost{}).Error
	if err != nil {
		return fmt.Errorf("delete the channel posts of a wa account: %w", err)
	}
	return nil
}

// purge removes the messages a scope names, answering how many rows went.
func (s *CSPurgeService) purge(scope within) (int, error) {
	var threads []uuid.UUID
	err := s.db.Model(&models.CSMessage{}).Scopes(scope).
		Distinct().Pluck("conversation_id", &threads).Error
	if err != nil {
		return 0, fmt.Errorf("find the threads a purge touches: %w", err)
	}

	var paths []string
	err = s.db.Model(&models.CSMessage{}).Scopes(scope).
		Where("media_path <> ''").Pluck("media_path", &paths).Error
	if err != nil {
		return 0, fmt.Errorf("find the attachments a purge removes: %w", err)
	}

	// The files go before the rows that name them, and one that will not come
	// off the disk stops the purge with nothing lost. The other order would
	// strand the file with its only pointer gone — a leak nothing can ever
	// find again — where failing here leaves a purge the admin can rerun.
	if err := s.removeFiles(paths); err != nil {
		return 0, err
	}

	res := s.db.Scopes(scope).Delete(&models.CSMessage{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete messages: %w", res.Error)
	}
	return int(res.RowsAffected), s.resummarise(threads)
}

// removeFiles takes attachments off the disk, reporting every path it could
// not rather than stopping at the first.
func (s *CSPurgeService) removeFiles(paths []string) error {
	var failures []error
	for _, rel := range paths {
		// A file somebody already deleted by hand counts as removed: the row
		// was pointing at it, and is about to stop.
		if err := os.Remove(filepath.Join(s.root, rel)); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("remove %s: %w", rel, err))
		}
	}
	return errors.Join(failures...)
}

// removeAvatars takes one number's customer profile photos off the disk. They
// sit under the media root like any attachment, and once the threads holding
// their paths are gone nothing would ever come back for them.
func (s *CSPurgeService) removeAvatars(accountID uuid.UUID) error {
	var paths []string
	err := s.db.Model(&models.CSConversation{}).
		Where("wa_account_id = ? AND avatar_path <> ''", accountID).
		Pluck("avatar_path", &paths).Error
	if err != nil {
		return fmt.Errorf("find the profile photos of a wa account: %w", err)
	}
	return s.removeFiles(paths)
}

// resummarise puts each thread's summary back in step with the messages it
// still holds. Without it the inbox goes on describing messages that are gone:
// a badge counting them, and the "belum dibalas" tab holding a thread with
// nothing left in it to reply to.
//
// One query per thread rather than one statement across all of them. The clamp
// is spelled differently by the two databases this runs on (LEAST against MIN),
// and a purge is a rare, deliberate act where being plainly right is worth
// more than being quick.
func (s *CSPurgeService) resummarise(threads []uuid.UUID) error {
	var failures []error
	for _, id := range threads {
		if err := s.resummariseOne(id); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *CSPurgeService) resummariseOne(id uuid.UUID) error {
	var readable int64
	err := s.db.Model(&models.CSMessage{}).
		Where("conversation_id = ? AND direction = ?", id, models.MessageIn).
		Count(&readable).Error
	if err != nil {
		return fmt.Errorf("count what a thread still has to read: %w", err)
	}

	fields := map[string]any{
		// Nothing records which messages were read, so the badge cannot be
		// recomputed — only held to what is actually there. It is clamped,
		// never raised: a purge must not invent unread messages.
		"unread_count": gorm.Expr(
			"CASE WHEN unread_count > ? THEN ? ELSE unread_count END", readable, readable),
		// An emptied thread has nothing to reply to, and the "belum dibalas"
		// tab reads exactly this column.
		"last_message_direction": "",
	}

	var last models.CSMessage
	err = s.db.Where("conversation_id = ?", id).Order("wa_timestamp DESC").First(&last).Error
	switch {
	case err == nil:
		fields["last_message_direction"] = last.Direction
		fields["last_message_at"] = last.WATimestamp
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("find what a thread now ends with: %w", err)
	}
	// last_message_at is left alone when nothing remains, so an emptied thread
	// keeps its place in the inbox instead of sinking to the bottom where the
	// CS who just cleared it would have to go hunting for it.

	// Not updateConversation: it reads no rows affected as a missing row, and
	// here a thread whose summary already said the right thing is a perfectly
	// ordinary outcome.
	err = s.db.Model(&models.CSConversation{}).Where("id = ?", id).Updates(fields).Error
	if err != nil {
		return fmt.Errorf("resummarise thread: %w", err)
	}
	return nil
}
