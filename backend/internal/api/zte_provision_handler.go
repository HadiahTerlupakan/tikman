package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

type zteJobReader interface {
	GetProvisioningJob(jobID uuid.UUID) (*models.ProvisioningJob, error)
}

var _ zteJobReader = (*services.JobService)(nil)

// ZTEProvisioner is the registration service boundary used by the HTTP layer.
type ZTEProvisioner interface {
	RegisterAndConfigure(ctx context.Context, req models.ZTEGPONRegisterRequest, userID uuid.UUID) (*models.ProvisioningJob, error)
	ConfigureExisting(ctx context.Context, ontID uuid.UUID, req models.ZTEGPONRegisterRequest, userID uuid.UUID) (*models.ProvisioningJob, error)
}

// ZTEProvisionHandler handles ZTE GPON registration and service configuration.
type ZTEProvisionHandler struct {
	provisioner ZTEProvisioner
	jobs        interface {
		GetProvisioningJob(jobID uuid.UUID) (*models.ProvisioningJob, error)
	}
}

// NewZTEProvisionHandler constructs a ZTE provisioning handler.
func NewZTEProvisionHandler(provisioner ZTEProvisioner, jobs ...interface {
	GetProvisioningJob(jobID uuid.UUID) (*models.ProvisioningJob, error)
}) *ZTEProvisionHandler {
	h := &ZTEProvisionHandler{provisioner: provisioner}
	if len(jobs) > 0 {
		h.jobs = jobs[0]
	}
	return h
}

// Register handles POST /api/v1/olts/:olt_id/gpon/register.
func (h *ZTEProvisionHandler) Register(c *gin.Context) {
	oltID, err := parseZTEID(c, "id")
	if err != nil {
		return
	}
	var req zteProvisionRequest
	if !bindZTERequest(c, &req) {
		return
	}
	if req.OLTID != uuid.Nil && req.OLTID != oltID {
		writeZTEError(c, http.StatusBadRequest, "VALIDATION_ERROR", "request OLT does not match URL")
		return
	}
	req.OLTID = oltID
	if !req.Confirm {
		writeZTEError(c, http.StatusBadRequest, "NOT_CONFIRMED", "provisioning requires explicit confirm=true")
		return
	}
	userID, _ := middleware.GetUserID(c)
	job, err := h.provisioner.RegisterAndConfigure(c.Request.Context(), req.request(), userID)
	if err != nil {
		writeZTEServiceError(c, err)
		return
	}
	writeZTEAccepted(c, job, jobONUID(job, req.ONUID), req.preview(jobONUID(job, req.ONUID)))
}

// ConfigureExisting handles POST /api/v1/onts/:ont_id/gpon/configure.
func (h *ZTEProvisionHandler) ConfigureExisting(c *gin.Context) {
	ontID, err := parseZTEID(c, "id")
	if err != nil {
		return
	}
	var req zteProvisionRequest
	if !bindZTERequest(c, &req) {
		return
	}
	if !req.Confirm {
		writeZTEError(c, http.StatusBadRequest, "NOT_CONFIRMED", "provisioning requires explicit confirm=true")
		return
	}
	userID, _ := middleware.GetUserID(c)
	job, err := h.provisioner.ConfigureExisting(c.Request.Context(), ontID, req.request(), userID)
	if err != nil {
		writeZTEServiceError(c, err)
		return
	}
	writeZTEAccepted(c, job, jobONUID(job, req.ONUID), req.preview(jobONUID(job, req.ONUID)))
}

// GetJob handles GET /api/v1/provision-jobs/:id for ZTE and generic jobs.
func (h *ZTEProvisionHandler) GetJob(c *gin.Context) {
	jobID, err := parseZTEID(c, "id")
	if err != nil {
		return
	}
	if h.jobs == nil {
		writeZTEError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "job service is not configured")
		return
	}
	job, err := h.jobs.GetProvisioningJob(jobID)
	if err != nil {
		writeZTEServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"job_id": job.ID, "status": job.Status, "ont_id": job.ONTID, "error_message": job.ErrorMessage}})
}

func bindZTERequest(c *gin.Context, req *zteProvisionRequest) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		writeZTEError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return false
	}
	return true
}

func parseZTEID(c *gin.Context, name string) (uuid.UUID, error) {
	value := c.Param(name)
	if value == "" {
		value = c.Param("olt_id")
	}
	if value == "" {
		value = c.Param("ont_id")
	}
	id, err := uuid.Parse(value)
	if err != nil {
		writeZTEError(c, http.StatusBadRequest, "INVALID_ID", "invalid "+name+" format")
	}
	return id, err
}

func writeZTEAccepted(c *gin.Context, job *models.ProvisioningJob, onuID int, commands []string) {
	c.JSON(http.StatusAccepted, zteProvisionResponse{JobID: job.ID, Status: job.Status, ONTID: job.ONTID, ONUID: onuID, Commands: commands})
}

func jobONUID(job *models.ProvisioningJob, requested int) int {
	if requested == 0 && job != nil && job.ONUID > 0 {
		return job.ONUID
	}
	return requested
}

func writeZTEServiceError(c *gin.Context, err error) {
	message := err.Error()
	code := "INTERNAL_ERROR"
	status := http.StatusInternalServerError
	switch {
	case strings.Contains(message, "C300 or C320"):
		code, status = "UNSUPPORTED_VENDOR", http.StatusBadRequest
	case strings.Contains(message, "required"), strings.Contains(message, "must be"), strings.Contains(message, "unsupported"), strings.Contains(message, "cannot"), strings.Contains(message, "already used"), strings.Contains(message, "does not match"):
		code, status = "VALIDATION_ERROR", http.StatusBadRequest
	case strings.Contains(message, "not found"):
		code, status = "NOT_FOUND", http.StatusNotFound
	}
	writeZTEError(c, status, code, message)
}

func writeZTEError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{Code: code, Error: message})
}
