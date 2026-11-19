package services

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

const (
	maxBRASRange = 31 * 24 * time.Hour
	// brasMatchWindow is how far from a wanted moment a sample may sit and
	// still stand for it; the poller writes one a minute.
	brasMatchWindow = 2 * time.Minute
	// A drop is a fall of the larger of an absolute floor and a share of the
	// earlier count, so a small BRAS's noise and a big one's churn both pass.
	brasDropFloor    = 100
	brasDropFraction = 0.05
)

// BRASDrop is one minute in which sessions fell sharply.
type BRASDrop struct {
	Time time.Time `json:"time"`
	Lost int       `json:"lost"`
}

// BRASView is what the BRAS page reads: the latest snapshot, the charted
// samples, and the counts judged against earlier samples. The RADIUS counts
// are nil when the history cannot answer them.
type BRASView struct {
	Snapshot         json.RawMessage           `json:"bras"`
	At               *time.Time                `json:"bras_at"`
	Samples          []models.RouterBRASSample `json:"samples"`
	Sessions15mAgo   *int                      `json:"sessions_15m_ago"`
	RadiusAccepts1h  *int64                    `json:"radius_accepts_1h"`
	RadiusRejects1h  *int64                    `json:"radius_rejects_1h"`
	RadiusTimeouts1h *int64                    `json:"radius_timeouts_1h"`
	Drops            []BRASDrop                `json:"drops"`
}

// BRASView returns a BRAS router's page data for the range, clamped to its
// last 31 days. A missing router and one that is not a BRAS are both
// gorm.ErrRecordNotFound.
func (s *RouterService) BRASView(id uuid.UUID, from, to time.Time) (*BRASView, error) {
	router, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if router.Role != models.RouterRoleBRAS {
		return nil, gorm.ErrRecordNotFound
	}
	if to.Sub(from) > maxBRASRange {
		from = to.Add(-maxBRASRange)
	}
	view := &BRASView{Snapshot: json.RawMessage(router.BRAS), At: router.BRASAt, Samples: []models.RouterBRASSample{}}
	err = s.db.Where("router_id = ? AND time >= ? AND time <= ?", id, from, to).Order("time").Find(&view.Samples).Error
	if err != nil {
		return nil, err
	}
	view.Drops = brasDrops(view.Samples)
	return view, s.fillBRASHistory(id, view)
}

// fillBRASHistory sets the figures that compare the latest sample with an
// earlier one. They read their own samples rather than the requested range,
// so a 1 h chart still shows an hourly RADIUS count.
func (s *RouterService) fillBRASHistory(id uuid.UUID, view *BRASView) error {
	var newest []models.RouterBRASSample
	if err := s.db.Where("router_id = ?", id).Order("time DESC").Limit(1).Find(&newest).Error; err != nil {
		return err
	}
	if len(newest) == 0 {
		return nil
	}
	latest := newest[0]
	var recent []models.RouterBRASSample
	err := s.db.Where("router_id = ? AND time >= ? AND time < ?", id,
		latest.Time.Add(-time.Hour-brasMatchWindow), latest.Time).Find(&recent).Error
	if err != nil {
		return err
	}
	if m := nearestBRASSample(recent, latest.Time.Add(-15*time.Minute)); m != nil {
		view.Sessions15mAgo = &m.Sessions
	}
	if m := nearestBRASSample(recent, latest.Time.Add(-time.Hour)); m != nil {
		view.RadiusAccepts1h = counterGrowth(latest.RadiusAccepts, m.RadiusAccepts)
		view.RadiusRejects1h = counterGrowth(latest.RadiusRejects, m.RadiusRejects)
		view.RadiusTimeouts1h = counterGrowth(latest.RadiusTimeouts, m.RadiusTimeouts)
	}
	return nil
}

// nearestBRASSample is the sample closest to want, within brasMatchWindow.
func nearestBRASSample(samples []models.RouterBRASSample, want time.Time) *models.RouterBRASSample {
	var best *models.RouterBRASSample
	var bestGap time.Duration
	for i := range samples {
		gap := samples[i].Time.Sub(want).Abs()
		if gap <= brasMatchWindow && (best == nil || gap < bestGap) {
			best, bestGap = &samples[i], gap
		}
	}
	return best
}

// counterGrowth is how much a cumulative counter grew; nil when it went
// backwards, which is a router reboot zeroing it, not a negative count.
func counterGrowth(latest, earlier int64) *int64 {
	if latest < earlier {
		return nil
	}
	n := latest - earlier
	return &n
}

// brasDrops finds the minutes sessions fell sharply. Samples more than two
// minutes apart are not compared: a poller outage would read as a fall.
func brasDrops(samples []models.RouterBRASSample) []BRASDrop {
	drops := []BRASDrop{}
	for i := 1; i < len(samples); i++ {
		prev, cur := samples[i-1], samples[i]
		if cur.Time.Sub(prev.Time) > brasMatchWindow {
			continue
		}
		lost := prev.Sessions - cur.Sessions
		if float64(lost) >= max(brasDropFloor, brasDropFraction*float64(prev.Sessions)) {
			drops = append(drops, BRASDrop{Time: cur.Time, Lost: lost})
		}
	}
	return drops
}
