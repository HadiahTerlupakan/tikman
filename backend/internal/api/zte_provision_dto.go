package api

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// zteProvisionRequest is the protected API payload for GPON registration and
// service configuration. PPPoE credentials are accepted only for execution.
type zteProvisionRequest struct {
	models.ZTEGPONRegisterRequest
	Confirm bool `json:"confirm"`
}

// zteProvisionResponse contains the queued job and a secret-free command preview.
type zteProvisionResponse struct {
	JobID    uuid.UUID `json:"job_id"`
	Status   string    `json:"status"`
	ONTID    uuid.UUID `json:"ont_id"`
	ONUID    int       `json:"onu_id"`
	Commands []string  `json:"commands"`
}

func (r zteProvisionRequest) request() models.ZTEGPONRegisterRequest {
	return r.ZTEGPONRegisterRequest
}

func (r zteProvisionRequest) preview(onuID int) []string {
	commands, err := services.BuildZTEGPONRegisterCommands(r.request(), onuID)
	if err != nil {
		return nil
	}
	return services.RedactZTECommands(commands)
}
