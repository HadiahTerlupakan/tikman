package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// WaitListItem is one wait as the report's list shows it: enough to find the
// thread it came from and to see how its figures were reached.
type WaitListItem struct {
	ID                  uuid.UUID            `json:"id"`
	ConversationID      uuid.UUID            `json:"conversation_id"`
	CustomerName        string               `json:"customer_name"`
	CustomerPhone       string               `json:"customer_phone"`
	ConversationDeleted bool                 `json:"conversation_deleted"`
	StartedAt           time.Time            `json:"started_at"`
	CustomerSentAt      time.Time            `json:"customer_sent_at"`
	EndedAt             time.Time            `json:"ended_at"`
	EndReason           models.WaitEndReason `json:"end_reason"`
	EndedBy             *uuid.UUID           `json:"ended_by,omitempty"`
	EndedByUsername     string               `json:"ended_by_username,omitempty"`
	TeamMinutes         *float64             `json:"team_minutes"`
	CountedMinutes      *float64             `json:"counted_minutes"`
	SystemDelayed       bool                 `json:"system_delayed"`
}

// WaitList is one page of waits, with how many there are in all.
type WaitList struct {
	Items []WaitListItem
	Total int64
}

// Waits lists the waits that ended within r, newest first. endedBy narrows the
// list to the waits one CS replied to or closed, which is all a CS may see.
func (s *CSPerformanceService) Waits(r ReportRange, endedBy *uuid.UUID, limit, offset int) (*WaitList, error) {
	within := func(db *gorm.DB) *gorm.DB {
		db = db.Where("ended_at >= ? AND ended_at < ?", r.From.UTC(), r.To.UTC())
		if endedBy != nil {
			db = db.Where("ended_by = ?", *endedBy)
		}
		return db
	}

	var total int64
	if err := s.db.Model(&models.CSWait{}).Scopes(within).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count waits: %w", err)
	}
	var page []models.CSWait
	err := s.db.Model(&models.CSWait{}).Scopes(within).
		Order("ended_at DESC").Limit(limit).Offset(offset).Find(&page).Error
	if err != nil {
		return nil, fmt.Errorf("load a page of waits: %w", err)
	}

	items, err := s.describeWaits(page)
	if err != nil {
		return nil, err
	}
	return &WaitList{Items: items, Total: total}, nil
}

// describeWaits turns stored waits into list items: the thread they belong to,
// who ended them, and their minutes.
func (s *CSPerformanceService) describeWaits(page []models.CSWait) ([]WaitListItem, error) {
	threads, err := s.threads(page)
	if err != nil {
		return nil, err
	}
	counted, err := s.countedByWait(page)
	if err != nil {
		return nil, err
	}
	names, err := s.usernames(enders(page))
	if err != nil {
		return nil, err
	}
	items := make([]WaitListItem, len(page))
	for i, w := range page {
		items[i] = waitListItem(w, threads, counted, names)
	}
	return items, nil
}

// threads loads the threads the waits point at. One that is missing was deleted
// with its number, and the wait outlives it on purpose.
func (s *CSPerformanceService) threads(page []models.CSWait) (map[uuid.UUID]models.CSConversation, error) {
	found := make(map[uuid.UUID]models.CSConversation, len(page))
	if len(page) == 0 {
		return found, nil
	}
	ids := make([]uuid.UUID, 0, len(page))
	for _, w := range page {
		ids = append(ids, w.ConversationID)
	}
	var threads []models.CSConversation
	err := s.db.Select("id", "customer_name", "customer_phone").Where("id IN ?", ids).Find(&threads).Error
	if err != nil {
		return nil, fmt.Errorf("load threads for waits: %w", err)
	}
	for _, thread := range threads {
		found[thread.ID] = thread
	}
	return found, nil
}

func enders(page []models.CSWait) []uuid.UUID {
	var ids []uuid.UUID
	for _, w := range page {
		if w.EndedBy != nil {
			ids = append(ids, *w.EndedBy)
		}
	}
	return ids
}

func waitListItem(w models.CSWait, threads map[uuid.UUID]models.CSConversation,
	counted map[uuid.UUID]float64, names map[uuid.UUID]string) WaitListItem {
	thread, exists := threads[w.ConversationID]
	item := WaitListItem{
		ID: w.ID, ConversationID: w.ConversationID,
		CustomerName: thread.CustomerName, CustomerPhone: thread.CustomerPhone,
		ConversationDeleted: !exists,
		StartedAt:           w.StartedAt, CustomerSentAt: w.CustomerSentAt,
		EndedAt: *w.EndedAt, EndReason: *w.EndReason, EndedBy: w.EndedBy,
		SystemDelayed: systemDelayed(w),
	}
	if w.EndedBy != nil {
		item.EndedByUsername = names[*w.EndedBy]
	}
	if answered(w) {
		team := teamMinutes(w)
		item.TeamMinutes = &team
	}
	if minutes, charged := counted[w.ID]; charged && !item.SystemDelayed {
		item.CountedMinutes = &minutes
	}
	return item
}
