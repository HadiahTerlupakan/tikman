package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSPerformanceService answers how fast the team answers customers, from the
// waits recorded as they happened (see cs_wait.go). It computes on read: the
// stored rows are facts, and the target, the session gap and the system-delay
// threshold are how the report reads them today.
type CSPerformanceService struct {
	db *gorm.DB
}

// NewCSPerformanceService constructs a CSPerformanceService.
func NewCSPerformanceService(db *gorm.DB) *CSPerformanceService {
	return &CSPerformanceService{db: db}
}

// TeamPerformance is what the customers of one period got.
type TeamPerformance struct {
	Replies            WaitStats `json:"replies"`
	ClosedWithoutReply int       `json:"closed_without_reply"`
	Abandoned          int       `json:"abandoned"`
	SystemDelayed      int       `json:"system_delayed"`
}

// AgentPerformance is one CS's row: the replies they sent, and the threads they
// closed without sending one.
type AgentPerformance struct {
	UserID             uuid.UUID `json:"user_id"`
	Username           string    `json:"username"`
	Replies            WaitStats `json:"replies"`
	ClosedWithoutReply int       `json:"closed_without_reply"`
}

// DayPerformance is one WIB day of the period.
type DayPerformance struct {
	Date    string    `json:"date"`
	Replies WaitStats `json:"replies"`
}

// WaitingNow is how many customers are waiting at this moment, and how long the
// one at the front has waited.
type WaitingNow struct {
	Count          int      `json:"count"`
	LongestMinutes *float64 `json:"longest_minutes"`
}

// PerformanceSummary is one report.
type PerformanceSummary struct {
	TargetMinutes int                `json:"target_minutes"`
	Team          TeamPerformance    `json:"team"`
	Agents        []AgentPerformance `json:"agents"`
	Days          []DayPerformance   `json:"days"`
	Waiting       WaitingNow         `json:"waiting"`
}

// Summary reports the waits that ended within r. onlyAgent narrows the per-CS
// rows to one person — what a CS sees of the others — and leaves the team's
// figures whole.
func (s *CSPerformanceService) Summary(r ReportRange, onlyAgent *uuid.UUID, now time.Time) (*PerformanceSummary, error) {
	waits, err := s.waitsEndedIn(r)
	if err != nil {
		return nil, err
	}
	counted, err := s.countedByWait(waits)
	if err != nil {
		return nil, err
	}
	agents, err := s.agentRows(waits, counted, onlyAgent)
	if err != nil {
		return nil, err
	}
	waiting, err := s.waitingNow(now)
	if err != nil {
		return nil, err
	}
	return &PerformanceSummary{
		TargetMinutes: csReplyTargetMinutes,
		Team:          teamRow(waits),
		Agents:        agents,
		Days:          dayRows(r, waits),
		Waiting:       waiting,
	}, nil
}

// waitsEndedIn loads the waits of one period, oldest first. A wait belongs to
// the period it ended in: an answer given this morning is this morning's work,
// whenever the customer wrote.
func (s *CSPerformanceService) waitsEndedIn(r ReportRange) ([]models.CSWait, error) {
	var waits []models.CSWait
	err := s.db.Where("ended_at >= ? AND ended_at < ?", r.From.UTC(), r.To.UTC()).
		Order("ended_at ASC").Find(&waits).Error
	if err != nil {
		return nil, fmt.Errorf("load waits: %w", err)
	}
	return waits, nil
}

// teamRow sums the period as the customers experienced it.
func teamRow(waits []models.CSWait) TeamPerformance {
	var row TeamPerformance
	var minutes []float64
	for _, w := range waits {
		// Counted whatever the ending, so it overlaps the two counts below. A
		// WhatsApp backlog lands days late and abandons waits in volume; if only
		// answered ones were counted here, that day would read as customers
		// giving up on a team that never had their messages.
		if systemDelayed(w) {
			row.SystemDelayed++
		}
		switch {
		case answered(w):
			if !systemDelayed(w) {
				minutes = append(minutes, teamMinutes(w))
			}
		case *w.EndReason == models.WaitClosed:
			row.ClosedWithoutReply++
		default:
			row.Abandoned++
		}
	}
	row.Replies = summarizeMinutes(minutes)
	return row
}

// dayRows gives every WIB day of the period a row, so a day nothing was
// answered on reads as a gap in the chart rather than as a missing day.
func dayRows(r ReportRange, waits []models.CSWait) []DayPerformance {
	byDate := map[string][]float64{}
	for _, w := range waits {
		if answered(w) && !systemDelayed(w) {
			byDate[reportDate(*w.EndedAt)] = append(byDate[reportDate(*w.EndedAt)], teamMinutes(w))
		}
	}
	dates := r.dates()
	rows := make([]DayPerformance, len(dates))
	for i, date := range dates {
		rows[i] = DayPerformance{Date: date, Replies: summarizeMinutes(byDate[date])}
	}
	return rows
}

// countedByWait works out, for every TikMan reply among the waits, the minutes
// charged to the CS who sent it.
func (s *CSPerformanceService) countedByWait(waits []models.CSWait) (map[uuid.UUID]float64, error) {
	byAgent := map[uuid.UUID][]models.CSWait{}
	for _, w := range waits {
		if w.EndedBy != nil && *w.EndReason == models.WaitReplied {
			byAgent[*w.EndedBy] = append(byAgent[*w.EndedBy], w)
		}
	}
	counted := map[uuid.UUID]float64{}
	for agent, replies := range byAgent {
		if err := s.chargeAgent(agent, replies, counted); err != nil {
			return nil, err
		}
	}
	return counted, nil
}

// chargeAgent charges one CS's replies. Their replies are read back
// csSessionLookback before the earliest of them, so a stretch of work that began
// before the report still starts where it really started — otherwise the first
// answer of every report would cost nothing.
func (s *CSPerformanceService) chargeAgent(agent uuid.UUID, replies []models.CSWait, counted map[uuid.UUID]float64) error {
	first, last := *replies[0].EndedAt, *replies[0].EndedAt
	for _, w := range replies {
		if w.EndedAt.Before(first) {
			first = *w.EndedAt
		}
		if w.EndedAt.After(last) {
			last = *w.EndedAt
		}
	}

	var times []time.Time
	err := s.db.Model(&models.CSWait{}).
		Where("end_reason = ? AND ended_by = ? AND ended_at >= ? AND ended_at <= ?",
			models.WaitReplied, agent, first.Add(-csSessionLookback).UTC(), last.UTC()).
		Order("ended_at ASC").Pluck("ended_at", &times).Error
	if err != nil {
		return fmt.Errorf("load replies for sessions: %w", err)
	}

	starts := sessionStarts(times)
	for _, w := range replies {
		i := sort.Search(len(times), func(i int) bool { return !times[i].Before(*w.EndedAt) })
		if i < len(times) {
			counted[w.ID] = countedMinutes(w, starts[i])
		}
	}
	return nil
}

// agentRows builds a row for every CS who replied or closed within the waits.
func (s *CSPerformanceService) agentRows(waits []models.CSWait, counted map[uuid.UUID]float64,
	onlyAgent *uuid.UUID) ([]AgentPerformance, error) {
	minutes := map[uuid.UUID][]float64{}
	closed := map[uuid.UUID]int{}
	for _, w := range waits {
		if w.EndedBy == nil || (onlyAgent != nil && *w.EndedBy != *onlyAgent) {
			continue
		}
		agent := *w.EndedBy
		if _, seen := minutes[agent]; !seen {
			minutes[agent] = nil
		}
		if *w.EndReason == models.WaitClosed {
			closed[agent]++
		} else if charged, ok := counted[w.ID]; ok && !systemDelayed(w) {
			minutes[agent] = append(minutes[agent], charged)
		}
	}
	return s.namedAgentRows(minutes, closed)
}

// namedAgentRows puts the CS's name on each row and sorts them by it, so the
// table reads the same way twice running.
func (s *CSPerformanceService) namedAgentRows(minutes map[uuid.UUID][]float64,
	closed map[uuid.UUID]int) ([]AgentPerformance, error) {
	ids := make([]uuid.UUID, 0, len(minutes))
	for id := range minutes {
		ids = append(ids, id)
	}
	names, err := s.usernames(ids)
	if err != nil {
		return nil, err
	}
	rows := make([]AgentPerformance, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, AgentPerformance{
			UserID: id, Username: names[id],
			Replies: summarizeMinutes(minutes[id]), ClosedWithoutReply: closed[id],
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Username < rows[j].Username })
	return rows, nil
}

// usernames answers the names of the CS ids given. A name missing here is a
// user who has been deleted; the row still stands, because the work did.
func (s *CSPerformanceService) usernames(ids []uuid.UUID) (map[uuid.UUID]string, error) {
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	var users []models.User
	if err := s.db.Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("load CS names: %w", err)
	}
	for _, user := range users {
		names[user.ID] = user.Username
	}
	return names, nil
}

// waitingNow counts the customers still waiting on threads that still exist. A
// wait left open on a deleted thread can never be answered, so counting it
// would leave a number nobody can ever bring down.
func (s *CSPerformanceService) waitingNow(now time.Time) (WaitingNow, error) {
	var started []time.Time
	err := s.db.Model(&models.CSWait{}).
		Joins("JOIN cs_conversations ON cs_conversations.id = cs_waits.conversation_id").
		Where("cs_waits.ended_at IS NULL").
		Order("cs_waits.started_at ASC").
		Pluck("cs_waits.started_at", &started).Error
	if err != nil {
		return WaitingNow{}, fmt.Errorf("load open waits: %w", err)
	}
	waiting := WaitingNow{Count: len(started)}
	if len(started) > 0 {
		longest := minutesBetween(started[0], now)
		waiting.LongestMinutes = &longest
	}
	return waiting, nil
}
