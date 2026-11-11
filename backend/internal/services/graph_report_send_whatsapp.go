package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// waTarget is a WhatsApp group checked to take the report, and whom it tags.
type waTarget struct {
	groupID  uuid.UUID
	mentions []string
}

func (s *GraphReportSender) prepareWA(report *models.GraphReport) (*waTarget, error) {
	group, err := s.groups.Get(*report.WAGroupID)
	if err != nil {
		return nil, fmt.Errorf("load group: %w", err)
	}
	if err := CheckGroupSendable(group); err != nil {
		return nil, err
	}
	mentions, err := s.mentions(group.ID, report.MentionAll)
	if err != nil {
		return nil, err
	}
	return &waTarget{groupID: group.ID, mentions: mentions}, nil
}

func (s *GraphReportSender) sendWA(
	ctx context.Context, report *models.GraphReport, wa *waTarget, images []ReportImage, now time.Time, daily bool,
) error {
	files, err := s.storeImages(report.CustomerName, images, now)
	if err != nil {
		return err
	}
	return s.queue(ctx, report, wa, images, files, now, daily)
}

// mentions answers who the report tags, read at send time so it follows the
// group as members come and go. The caption is left as written: WhatsApp
// notifies every tagged member whether or not the text names them.
func (s *GraphReportSender) mentions(groupID uuid.UUID, mentionAll bool) ([]string, error) {
	if !mentionAll {
		return nil, nil
	}
	jids, err := s.groups.MemberJIDsOtherThanUs(groupID)
	if err != nil {
		return nil, err
	}
	// Failing beats sending untagged: a report quietly missing its tags
	// reads as sent while nobody muted was told.
	if len(jids) > maxMentions {
		return nil, ErrGraphReportTooManyMentions
	}
	return jids, nil
}

// storeImages writes a report's images where the wa process reads
// attachments, under the <year>/<month>/<uuid> layout the inbox's uploads use.
// A write that fails removes the images already written, as nothing is queued.
func (s *GraphReportSender) storeImages(customer string, images []ReportImage, now time.Time) ([]MediaFile, error) {
	dir := filepath.Join(now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(filepath.Join(s.mediaRoot, dir), 0o750); err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}
	base := reportFileBase(customer, now)
	files := make([]MediaFile, 0, len(images))
	for i, image := range images {
		rel := filepath.Join(dir, uuid.NewString()+".png")
		if err := os.WriteFile(filepath.Join(s.mediaRoot, rel), image.PNG, 0o640); err != nil {
			s.removeImages(files)
			return nil, fmt.Errorf("write report image: %w", err)
		}
		files = append(files, MediaFile{
			Path: rel, Mime: "image/png", Filename: imageFilename(base, i, len(images)), Size: int64(len(image.PNG)),
		})
	}
	return files, nil
}

func (s *GraphReportSender) removeImages(files []MediaFile) {
	for _, file := range files {
		_ = os.Remove(filepath.Join(s.mediaRoot, file.Path))
	}
}

// queue writes the messages and the group's sent-on date in one
// transaction, so a restart cannot see the images queued but the group still
// owed the day's report. Every image is captioned with its ports; only the
// last carries the tags, as tags on every image would notify each member once
// per image.
func (s *GraphReportSender) queue(
	ctx context.Context, report *models.GraphReport, wa *waTarget, images []ReportImage, files []MediaFile,
	now time.Time, daily bool,
) error {
	members, _ := s.groups.MembersOtherThanUs(wa.groupID)
	outs := make([]GroupOutgoing, len(files))
	for i := range files {
		outs[i] = GroupOutgoing{
			GroupID: wa.groupID, SenderName: GraphReportSenderName, Kind: models.MessageKindImage,
			Body:  GraphImageCaption(report.Message, report.CustomerName, images[i].Ports),
			Media: &files[i], MemberCount: members,
		}
	}
	outs[len(outs)-1].Mentions = wa.mentions
	fields := map[string]any{"last_attempt_at": now}
	if daily {
		fields["wa_sent_on"] = wibDate(now)
	}
	msgs, err := s.messages.QueueBatch(outs, func(tx *gorm.DB) error {
		return tx.Model(&models.GraphReport{}).Where("id = ?", report.ID).Updates(fields).Error
	})
	if err != nil {
		s.removeImages(files)
		return fmt.Errorf("queue graph report: %w", err)
	}
	if s.notifier != nil {
		s.notifier.GroupMessagesQueued(ctx, msgs)
	}
	return nil
}
