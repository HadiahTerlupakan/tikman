package api

import (
	"github.com/tikman/olt-provisioning/internal/services"
)

type OLTHandler struct {
	service          *services.OLTService
	validatorService *services.OLTValidatorService
	auditService     *services.AuditService
	ontService       *services.ONTService
	pollJobService   *services.PollJobService
}

func NewOLTHandler(service *services.OLTService, validatorService *services.OLTValidatorService, auditService *services.AuditService, ontService *services.ONTService, pollJobService *services.PollJobService) *OLTHandler {
	return &OLTHandler{
		service:          service,
		validatorService: validatorService,
		auditService:     auditService,
		ontService:       ontService,
		pollJobService:   pollJobService,
	}
}
