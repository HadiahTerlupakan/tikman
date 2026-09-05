package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ZTESnapshotter captures state used to verify or roll back a ZTE change.
type ZTESnapshotter interface {
	CaptureBeforeSnapshot(models.ONT) (*ConfigSnapshot, error)
	CaptureAfterSnapshot(models.ONT) (*ConfigSnapshot, error)
}

// ZTERollbacker restores a failed ZTE change.
type ZTERollbacker interface {
	RollbackToSnapshot(context.Context, models.ONT, *ConfigSnapshot) error
}

type zteOLTRollbacker interface {
	RollbackToSnapshotForOLT(context.Context, models.OLT, models.ONT, *ConfigSnapshot) error
}

// ZTEGPONRegisterService coordinates ZTE registration and Internet service jobs.
type ZTEGPONRegisterService struct {
	db               *gorm.DB
	jobs             *JobService
	snapshot         ZTESnapshotter
	commanderFactory CommanderFactory
	rollback         ZTERollbacker
	logger           *zap.Logger
	encryptionKey    []byte
}

// WithEncryptionKey lets the service seal a PPPoE password when it records the
// service a job applied. Without one the service is still stored, minus the
// password, which the next running-config read supplies.
func (s *ZTEGPONRegisterService) WithEncryptionKey(key string) *ZTEGPONRegisterService {
	s.encryptionKey = []byte(key)
	return s
}

// NewZTEGPONRegisterService constructs the ZTE registration service.
func NewZTEGPONRegisterService(db *gorm.DB, jobs *JobService, snapshot ZTESnapshotter, factory CommanderFactory, rollback ZTERollbacker, logger *zap.Logger) *ZTEGPONRegisterService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ZTEGPONRegisterService{db: db, jobs: jobs, snapshot: snapshot, commanderFactory: factory, rollback: rollback, logger: logger}
}

// RegisterAndConfigure registers a new ONU and applies its Internet service.
func canonicalZTESerial(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (s *ZTEGPONRegisterService) RegisterAndConfigure(ctx context.Context, req models.ZTEGPONRegisterRequest, userID uuid.UUID) (*models.ProvisioningJob, error) {
	olt, err := s.loadOLT(req.OLTID)
	if err != nil {
		return nil, err
	}
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return nil, err
	}
	// Checked before the transaction, and so before any command reaches the
	// OLT: a bad ODP port should cost the operator a corrected form, not an ONU
	// half-registered on hardware.
	if err := validateRegisterODP(s.db.WithContext(ctx), req); err != nil {
		return nil, err
	}

	// The position is reserved in the same transaction as the ONT row. The
	// composite unique index is the final arbiter when two registrations race.
	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	var ont models.ONT
	var onuID int
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		onuID, err = resolveZTEONUIDLocked(ctx, tx, req.OLTID, req.Card, req.PON, req.ONUID)
		if err != nil {
			return err
		}
		ont = models.ONT{OLTID: req.OLTID, PortID: req.PON, ONTID: onuID, Slot: &req.Card, SerialNumber: req.SerialNumber, Name: req.Name, Description: req.Description, DeviceType: req.ONUType, Status: models.ONTStatusUnknown, ODPID: req.ODPID, ODPPort: req.ODPPort}
		if err := tx.Create(&ont).Error; err != nil {
			return reserveONUError(tx, req.SerialNumber, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	snap := &ConfigSnapshot{OntID: ont.ID, ZTE: &ZTESnapshot{SerialNumber: ont.SerialNumber, Name: ont.Name, DeviceType: ont.DeviceType, ServiceMode: "gpon-register"}}
	return s.executeJob(ctx, req, olt, ont, snap, userID, true)
}

// ConfigureExisting applies the Internet service to an already registered ONU.
func (s *ZTEGPONRegisterService) ConfigureExisting(ctx context.Context, ontID uuid.UUID, req models.ZTEGPONRegisterRequest, userID uuid.UUID) (*models.ProvisioningJob, error) {
	var ont models.ONT
	if err := s.db.WithContext(ctx).First(&ont, "id = ?", ontID).Error; err != nil {
		return nil, fmt.Errorf("load ONT: %w", err)
	}
	if req.OLTID != ont.OLTID {
		return nil, fmt.Errorf("request OLT does not own ONT")
	}
	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	olt, err := s.loadOLT(ont.OLTID)
	if err != nil {
		return nil, err
	}
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return nil, err
	}
	if ont.Slot == nil || *ont.Slot != req.Card || ont.PortID != req.PON {
		return nil, fmt.Errorf("request position does not match ONT")
	}
	if req.ONUIDMode == models.ZTEONUIDCustom && req.ONUID != ont.ONTID {
		return nil, fmt.Errorf("request ONU ID does not match ONT")
	}
	if s.snapshot == nil {
		return nil, fmt.Errorf("snapshot service not configured")
	}
	before, err := s.snapshot.CaptureBeforeSnapshot(ont)
	if err != nil {
		return nil, fmt.Errorf("capture before snapshot: %w", err)
	}
	return s.executeJob(ctx, req, olt, ont, before, userID, false)
}

// PreviewRegister returns the ONU ID the allocator would assign and the
// commands the OLT would receive, without writing anything. The preview an
// operator approves has to come from the same builder that runs, or it is not
// a preview of what happens.
func (s *ZTEGPONRegisterService) PreviewRegister(ctx context.Context, req models.ZTEGPONRegisterRequest) (int, []string, error) {
	olt, err := s.loadOLT(req.OLTID)
	if err != nil {
		return 0, nil, err
	}
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return 0, nil, err
	}

	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	onuID, err := ResolveZTEONUID(ctx, s.db, req.OLTID, req.Card, req.PON, req.ONUID)
	if err != nil {
		return 0, nil, err
	}

	commands, err := BuildZTEGPONRegisterCommands(req, onuID)
	if err != nil {
		return 0, nil, err
	}
	return onuID, commands, nil
}

// PreviewConfigure is the same for an ONT that is already registered, whose
// position and ONU ID are fixed.
func (s *ZTEGPONRegisterService) PreviewConfigure(ctx context.Context, ontID uuid.UUID, req models.ZTEGPONRegisterRequest) (int, []string, error) {
	var ont models.ONT
	if err := s.db.WithContext(ctx).First(&ont, "id = ?", ontID).Error; err != nil {
		return 0, nil, fmt.Errorf("load ONT: %w", err)
	}

	olt, err := s.loadOLT(ont.OLTID)
	if err != nil {
		return 0, nil, err
	}
	req.OLTID = ont.OLTID
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return 0, nil, err
	}

	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	commands, err := BuildZTEGPONServiceCommands(req, ont.ONTID)
	if err != nil {
		return 0, nil, err
	}
	return ont.ONTID, commands, nil
}
