package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// DistributionService owns the passive plant between a PON port and a
// subscriber's drop cable: cabinets, the ports feeding them, and the
// distribution boxes a drop lands in.
//
// The rules here are also written as constraints in 39_add_distribution_plant,
// because the database is what a bulk import or a stray query answers to. They
// live here as well so an operator gets a sentence rather than a constraint
// name, and so the SQLite tests can reach them.
type DistributionService struct {
	db *gorm.DB
}

// NewDistributionService constructs a DistributionService.
func NewDistributionService(db *gorm.DB) *DistributionService {
	return &DistributionService{db: db}
}

// ODCInput is what an operator states when recording a cabinet.
type ODCInput struct {
	SiteID    uuid.UUID
	Code      string
	Latitude  *float64
	Longitude *float64
	Address   string
	Notes     string
}

// ODCFeedInput is one PON port supplying a cabinet, with its splitter.
type ODCFeedInput struct {
	ODCID           uuid.UUID
	OLTID           uuid.UUID
	Slot            int
	PortID          int
	SplitterOutputs int
	Notes           string
}

// ODPInput is a distribution box and the one parent it hangs off: either
// ODCID, or the OLTID/Slot/PortID triple.
type ODPInput struct {
	Code      string
	PortCount int
	Latitude  *float64
	Longitude *float64
	Address   string
	Notes     string

	ODCID  *uuid.UUID
	OLTID  *uuid.UUID
	Slot   *int
	PortID *int
}

// CreateODC records a cabinet. Its feeds are added separately, because a
// cabinet can be fed by more than one PON port.
func (s *DistributionService) CreateODC(in ODCInput) (*models.ODC, error) {
	if in.Code == "" {
		return nil, fmt.Errorf("%w: the cabinet needs a code", ErrValidation)
	}
	odc := &models.ODC{
		SiteID: in.SiteID, Code: in.Code,
		Latitude: in.Latitude, Longitude: in.Longitude,
		Address: in.Address, Notes: in.Notes,
	}
	if err := s.db.Create(odc).Error; err != nil {
		return nil, err
	}
	return odc, nil
}

// CreateODCWithFeeds records a cabinet and the PON ports feeding it together.
//
// One transaction, because a cabinet saved without the feed that was refused
// would stand fed by nothing, and there is no screen to add the feed from
// afterwards. A cabinet with no feeds at all is fine: recording where it stands
// before its feeder is spliced is ordinary field order.
func (s *DistributionService) CreateODCWithFeeds(
	in ODCInput, feeds []ODCFeedInput,
) (*models.ODC, error) {
	if in.Code == "" {
		return nil, fmt.Errorf("%w: the cabinet needs a code", ErrValidation)
	}

	odc := &models.ODC{
		SiteID: in.SiteID, Code: in.Code,
		Latitude: in.Latitude, Longitude: in.Longitude,
		Address: in.Address, Notes: in.Notes,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(odc).Error; err != nil {
			return err
		}
		for _, feed := range feeds {
			feed.ODCID = odc.ID
			if _, err := addODCFeed(tx, feed); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return odc, nil
}

// AddODCFeed records one PON port supplying a cabinet that already exists.
func (s *DistributionService) AddODCFeed(in ODCFeedInput) (*models.ODCFeed, error) {
	return addODCFeed(s.db, in)
}

// addODCFeed holds the rule in one place so it applies whether a feed arrives
// with its cabinet or later, and inside a transaction or outside one.
func addODCFeed(db *gorm.DB, in ODCFeedInput) (*models.ODCFeed, error) {
	if in.SplitterOutputs <= 0 {
		return nil, fmt.Errorf("%w: a splitter has outputs", ErrValidation)
	}

	var taken models.ODCFeed
	err := db.Where("olt_id = ? AND slot = ? AND port_id = ?",
		in.OLTID, in.Slot, in.PortID).First(&taken).Error
	if err == nil {
		return nil, fmt.Errorf("%w: PON port %d/%d already feeds another cabinet",
			ErrValidation, in.Slot, in.PortID)
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	feed := &models.ODCFeed{
		ODCID: in.ODCID, OLTID: in.OLTID, Slot: in.Slot, PortID: in.PortID,
		SplitterOutputs: in.SplitterOutputs, Notes: in.Notes,
	}
	if err := db.Create(feed).Error; err != nil {
		return nil, err
	}
	return feed, nil
}

// CreateODP records a distribution box under exactly one parent.
func (s *DistributionService) CreateODP(in ODPInput) (*models.ODP, error) {
	if in.Code == "" {
		return nil, fmt.Errorf("%w: the distribution box needs a code", ErrValidation)
	}
	if in.PortCount <= 0 {
		return nil, fmt.Errorf("%w: a splitter has outputs", ErrValidation)
	}
	if err := validateODPParent(in); err != nil {
		return nil, err
	}

	odp := &models.ODP{
		Code: in.Code, PortCount: in.PortCount,
		Latitude: in.Latitude, Longitude: in.Longitude,
		Address: in.Address, Notes: in.Notes,
		ODCID: in.ODCID, OLTID: in.OLTID, Slot: in.Slot, PortID: in.PortID,
	}
	if err := s.db.Create(odp).Error; err != nil {
		return nil, err
	}
	return odp, nil
}

// validateODPParent enforces the one rule the shape of the table cannot: a box
// hangs off a cabinet or off a PON port, never both and never neither.
func validateODPParent(in ODPInput) error {
	underCabinet := in.ODCID != nil
	underPort := in.OLTID != nil || in.Slot != nil || in.PortID != nil

	if underCabinet && underPort {
		return fmt.Errorf("%w: a distribution box hangs off one parent, a cabinet or a PON port, not both", ErrValidation)
	}
	if !underCabinet && !underPort {
		return fmt.Errorf("%w: a distribution box hangs off one parent, a cabinet or a PON port", ErrValidation)
	}
	if underPort && (in.OLTID == nil || in.Slot == nil || in.PortID == nil) {
		return fmt.Errorf("%w: a PON port parent needs the OLT, the slot and the port", ErrValidation)
	}
	return nil
}

// AssignONT lands a subscriber's drop on a port of a distribution box. Moving
// the same subscriber to another port is ordinary field work; landing on a port
// another subscriber holds is not.
func (s *DistributionService) AssignONT(ontID, odpID uuid.UUID, port int) error {
	if err := ValidateODPPort(s.db, odpID, port, ontID); err != nil {
		return err
	}
	return s.db.Model(&models.ONT{}).Where("id = ?", ontID).
		Updates(map[string]interface{}{"odp_id": odpID, "odp_port": port}).Error
}

// ValidateODPPort refuses a port that the box does not have, or that another
// subscriber already occupies. Pass uuid.Nil as excluding when the ONT does not
// exist yet, as it does not while an ONU is being registered.
//
// A racing pair of assignments can still both pass this; the composite unique
// index on (odp_id, odp_port) is the final arbiter.
func ValidateODPPort(db *gorm.DB, odpID uuid.UUID, port int, excluding uuid.UUID) error {
	var odp models.ODP
	if err := db.First(&odp, "id = ?", odpID).Error; err != nil {
		return err
	}
	if port < 1 || port > odp.PortCount {
		return fmt.Errorf("%w: %s has %d ports, so port %d does not exist",
			ErrValidation, odp.Code, odp.PortCount, port)
	}

	var holder models.ONT
	err := db.Where("odp_id = ? AND odp_port = ? AND id <> ?", odpID, port, excluding).
		First(&holder).Error
	if err == nil {
		return fmt.Errorf("%w: port %d is taken by %s",
			ErrValidation, port, holder.SerialNumber)
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}

// UnassignONT takes a subscriber off its port, freeing it for someone else.
func (s *DistributionService) UnassignONT(ontID uuid.UUID) error {
	return s.db.Model(&models.ONT{}).Where("id = ?", ontID).
		Updates(map[string]interface{}{"odp_id": nil, "odp_port": nil}).Error
}
