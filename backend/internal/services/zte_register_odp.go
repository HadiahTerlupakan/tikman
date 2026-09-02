package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// validateRegisterODP checks the plant position a registration asks for.
//
// The database refuses one half of a pairing, so the request must carry both or
// neither; saying so here names which half is missing.
func validateRegisterODP(db *gorm.DB, req models.ZTEGPONRegisterRequest) error {
	if req.ODPID == nil && req.ODPPort == nil {
		return nil
	}
	if req.ODPID == nil {
		return fmt.Errorf("%w: a port was given without an ODP", ErrValidation)
	}
	if req.ODPPort == nil {
		return fmt.Errorf("%w: an ODP was given without a port", ErrValidation)
	}
	return ValidateODPPort(db, *req.ODPID, *req.ODPPort, uuid.Nil)
}
