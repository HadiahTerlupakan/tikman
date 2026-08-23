package api

import (
	"github.com/tikman/olt-provisioning/internal/services"
)

type OLTHandler struct {
	service          *services.OLTService
	validatorService *services.OLTValidatorService
	auditService     *services.AuditService
	ontService       *services.ONTService
}

func NewOLTHandler(service *services.OLTService, validatorService *services.OLTValidatorService, auditService *services.AuditService, ontService *services.ONTService) *OLTHandler {
	return &OLTHandler{
		service:          service,
		validatorService: validatorService,
		auditService:     auditService,
		ontService:       ontService,
	}
}
