package api

import (
	"github.com/tikman/olt-provisioning/internal/services"
)

type OLTHandler struct {
	service          *services.OLTService
	validatorService *services.OLTValidatorService
	auditService     *services.AuditService
}

func NewOLTHandler(service *services.OLTService, validatorService *services.OLTValidatorService, auditService *services.AuditService) *OLTHandler {
	return &OLTHandler{
		service:          service,
		validatorService: validatorService,
		auditService:     auditService,
	}
}
