package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// ConfigTemplateHandler handles config template HTTP requests.
type ConfigTemplateHandler struct {
	service *services.ConfigTemplateService
}

// NewConfigTemplateHandler creates a new config template handler.
func NewConfigTemplateHandler(service *services.ConfigTemplateService) *ConfigTemplateHandler {
	return &ConfigTemplateHandler{service: service}
}

// configTemplateResponse is the wire representation of a config template.
type configTemplateResponse struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Vendor       string                 `json:"vendor"`
	ConfigFields map[string]interface{} `json:"config_fields"`
	IsDefault    bool                   `json:"is_default"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// List handles GET /api/v1/config-templates
func (h *ConfigTemplateHandler) List(c *gin.Context) {
	templates, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Code: "LIST_FAILED", Error: err.Error()})
		return
	}

	responses := make([]configTemplateResponse, 0, len(templates))
	for i := range templates {
		responses = append(responses, toConfigTemplateResponse(&templates[i]))
	}

	c.JSON(http.StatusOK, gin.H{"data": responses, "total": len(responses)})
}

// GetByID handles GET /api/v1/config-templates/:id
func (h *ConfigTemplateHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid template ID format"})
		return
	}

	template, err := h.service.GetByID(id)
	if err != nil {
		status, code := mapTemplateError(err)
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toConfigTemplateResponse(template)})
}

// Create handles POST /api/v1/config-templates
func (h *ConfigTemplateHandler) Create(c *gin.Context) {
	var req services.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_REQUEST", Error: err.Error()})
		return
	}

	userID, _ := middleware.GetUserID(c)
	template, err := h.service.Create(
		req.Name,
		req.Description,
		req.Vendor,
		req.ConfigFields,
		req.IsDefault,
		userID,
	)
	if err != nil {
		status, code := mapTemplateError(err)
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": toConfigTemplateResponse(template)})
}

// Update handles PUT /api/v1/config-templates/:id
func (h *ConfigTemplateHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid template ID format"})
		return
	}

	var req services.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_REQUEST", Error: err.Error()})
		return
	}

	userID, _ := middleware.GetUserID(c)
	template, err := h.service.Update(
		id,
		req.Name,
		req.Description,
		req.Vendor,
		req.ConfigFields,
		req.IsDefault,
		userID,
	)
	if err != nil {
		status, code := mapTemplateError(err)
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toConfigTemplateResponse(template)})
}

// Delete handles DELETE /api/v1/config-templates/:id
func (h *ConfigTemplateHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid template ID format"})
		return
	}

	if err := h.service.Delete(id); err != nil {
		status, code := mapTemplateError(err)
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config template deleted"})
}

// mapTemplateError converts service errors into HTTP status codes.
func mapTemplateError(err error) (int, string) {
	switch {
	case strings.Contains(err.Error(), "not found"):
		return http.StatusNotFound, "NOT_FOUND"
	case strings.Contains(err.Error(), "referenced by"):
		return http.StatusConflict, "IN_USE"
	case strings.Contains(err.Error(), "must be one of"),
		strings.Contains(err.Error(), "must be between"),
		strings.Contains(err.Error(), "must be unique"):
		return http.StatusBadRequest, "VALIDATION_ERROR"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

// toConfigTemplateResponse converts a model to its wire representation.
func toConfigTemplateResponse(t *models.ConfigTemplate) configTemplateResponse {
	fields := make(map[string]interface{})
	if len(t.ConfigFields) > 0 {
		_ = json.Unmarshal(t.ConfigFields, &fields)
	}

	createdAt := ""
	if !t.CreatedAt.IsZero() {
		createdAt = t.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	updatedAt := ""
	if !t.UpdatedAt.IsZero() {
		updatedAt = t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return configTemplateResponse{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Vendor:       t.Vendor,
		ConfigFields: fields,
		IsDefault:    t.IsDefault,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}
