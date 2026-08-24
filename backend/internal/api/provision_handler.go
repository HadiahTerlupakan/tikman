package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// ProvisionHandler handles single and batch ONT provisioning HTTP requests.
type ProvisionHandler struct {
	provisioner *services.OntProvisioningService
	batch       *services.BatchExecutor
}

// NewProvisionHandler creates a new provisioning handler.
func NewProvisionHandler(provisioner *services.OntProvisioningService, batch *services.BatchExecutor) *ProvisionHandler {
	return &ProvisionHandler{provisioner: provisioner, batch: batch}
}

// provisionRequest is the payload for single ONT provisioning.
type provisionRequest struct {
	TemplateID   *uuid.UUID             `json:"template_id"`
	ManualConfig map[string]interface{} `json:"manual_config"`
	Confirm      bool                   `json:"confirm"`
}

// provisionJobResponse is the wire representation of a provisioning job.
type provisionJobResponse struct {
	ID           uuid.UUID  `json:"id"`
	ONTID        uuid.UUID  `json:"ont_id"`
	TemplateID   *uuid.UUID `json:"template_id,omitempty"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    string     `json:"created_at"`
	CompletedAt  string     `json:"completed_at,omitempty"`
}

// ProvisionOnt handles POST /api/v1/onts/:id/provision
func (h *ProvisionHandler) ProvisionOnt(c *gin.Context) {
	ontID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid ONT ID format"})
		return
	}

	var req provisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_REQUEST", Error: err.Error()})
		return
	}

	if !req.Confirm {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "NOT_CONFIRMED", Error: "provisioning requires explicit confirm=true"})
		return
	}

	userID, _ := middleware.GetUserID(c)
	config := services.ProvisionConfig{
		TemplateID:   req.TemplateID,
		ManualConfig: req.ManualConfig,
		Confirm:      req.Confirm,
	}

	result, err := h.provisioner.ProvisionOnt(c.Request.Context(), ontID, userID, config)
	if err != nil {
		status, code := mapProvisionError(err)
		c.JSON(status, gin.H{
			"code":   code,
			"error":  err.Error(),
			"job_id": jobIDFromResult(result),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":  result.Job.ID,
		"status":  result.Job.Status,
		"message": "provisioning completed",
	})
}

// GetProvisionJob handles GET /api/v1/provision-jobs/:id
func (h *ProvisionHandler) GetProvisionJob(c *gin.Context) {
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid job ID format"})
		return
	}

	job, err := h.provisioner.GetProvisioningJob(jobID)
	if err != nil {
		status, code := mapProvisionError(err)
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toProvisionJobResponse(job)})
}

// ListProvisionJobsByONT handles GET /api/v1/onts/:id/provision-jobs
func (h *ProvisionHandler) ListProvisionJobsByONT(c *gin.Context) {
	ontID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid ONT ID format"})
		return
	}

	limit, offset := paginationParams(c)
	jobs, total, err := h.provisioner.ListProvisioningJobsByONT(ontID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Code: "LIST_FAILED", Error: err.Error()})
		return
	}

	responses := make([]provisionJobResponse, 0, len(jobs))
	for i := range jobs {
		responses = append(responses, toProvisionJobResponse(&jobs[i]))
	}

	c.JSON(http.StatusOK, gin.H{"data": responses, "total": total})
}

// batchProvisionRequest is the payload for batch provisioning.
type batchProvisionRequest struct {
	TemplateID   *uuid.UUID             `json:"template_id" binding:"required"`
	ONTIDs       []uuid.UUID            `json:"ont_ids" binding:"required,min=1"`
	ManualConfig map[string]interface{} `json:"manual_config"`
	Confirm      bool                   `json:"confirm"`
}

// BatchProvision handles POST /api/v1/batch-provision
func (h *ProvisionHandler) BatchProvision(c *gin.Context) {
	var req batchProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_REQUEST", Error: err.Error()})
		return
	}

	if !req.Confirm {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "NOT_CONFIRMED", Error: "batch provisioning requires explicit confirm=true"})
		return
	}

	userID, _ := middleware.GetUserID(c)
	config := services.BatchConfig{
		TemplateID:   req.TemplateID,
		ManualConfig: req.ManualConfig,
		UserID:       userID,
		ONTIDs:       req.ONTIDs,
	}

	result, err := h.batch.Execute(c.Request.Context(), config)
	if err != nil {
		status, code := mapProvisionError(err)
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":      result.Job.ID,
		"status":      result.Job.Status,
		"succeeded":   result.Succeeded,
		"failed":      result.Failed,
		"rolled_back": result.RolledBack,
		"details":     result.Details,
	})
}

// GetBatchJob handles GET /api/v1/batch-jobs/:id
func (h *ProvisionHandler) GetBatchJob(c *gin.Context) {
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid job ID format"})
		return
	}

	job, err := h.batch.GetBatchResult(jobID)
	if err != nil {
		status, code := mapProvisionError(err)
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": job})
}

// mapProvisionError converts provisioning service errors into HTTP status codes.
func mapProvisionError(err error) (int, string) {
	switch {
	case strings.Contains(err.Error(), "not found"):
		return http.StatusNotFound, "NOT_FOUND"
	case strings.Contains(err.Error(), "already running"):
		return http.StatusConflict, "CONFLICT"
	case strings.Contains(err.Error(), "requires a template"),
		strings.Contains(err.Error(), "at least one ONT"):
		return http.StatusBadRequest, "VALIDATION_ERROR"
	case strings.Contains(err.Error(), "failed to capture before snapshot"):
		return http.StatusBadGateway, "SNMP_UNREACHABLE"
	case strings.Contains(err.Error(), "provision execution failed"),
		strings.Contains(err.Error(), "config drift detected"):
		return http.StatusInternalServerError, "PROVISION_FAILED"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

// jobIDFromResult safely extracts the job ID from a possibly-nil result.
func jobIDFromResult(result *services.ProvisionResult) *uuid.UUID {
	if result == nil || result.Job == nil {
		return nil
	}
	return &result.Job.ID
}

// paginationParams extracts limit/offset query params with sane defaults.
func paginationParams(c *gin.Context) (limit, offset int) {
	limit = 20
	offset = 0
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && v >= 0 {
		offset = v
	}
	return
}

// toProvisionJobResponse converts a model to its wire representation.
func toProvisionJobResponse(j *models.ProvisioningJob) provisionJobResponse {
	resp := provisionJobResponse{
		ID:         j.ID,
		ONTID:      j.ONTID,
		TemplateID: j.TemplateID,
		Status:     j.Status,
	}
	if j.ErrorMessage != nil {
		resp.ErrorMessage = *j.ErrorMessage
	}
	if !j.CreatedAt.IsZero() {
		resp.CreatedAt = j.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if j.CompletedAt != nil {
		resp.CompletedAt = j.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return resp
}
