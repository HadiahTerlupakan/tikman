package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The write half of /api/v1/onts.

// Create handles POST /api/v1/onts
func (h *ONTHandler) Create(c *gin.Context) {
	var req CreateONTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "VALIDATION_ERROR",
			Error: err.Error(),
		})
		return
	}

	ont := &models.ONT{
		OLTID:        req.OLTID,
		PortID:       req.PortID,
		ONTID:        req.ONTID,
		SerialNumber: req.SerialNumber,
		Description:  req.Description,
		Status:       req.Status,
		Phone:        req.Phone,
	}

	if err := h.ontService.Create(ont); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "CREATE_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"create",
		"ont",
		ont.ID,
		nil,
		map[string]interface{}{
			"olt_id":        ont.OLTID,
			"port_id":       ont.PortID,
			"ont_id":        ont.ONTID,
			"serial_number": ont.SerialNumber,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusCreated, ToONTResponse(ont))
}

// Update handles PUT /api/v1/onts/:id
func (h *ONTHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	var req UpdateONTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "VALIDATION_ERROR",
			Error: err.Error(),
		})
		return
	}

	// Read before the write, so the audit entry can say what changed.
	oldONT, _ := h.ontService.GetByID(id)

	ont, err := h.ontService.Update(id, ontUpdates(req))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "UPDATE_FAILED",
			Error: err.Error(),
		})
		return
	}

	h.logONTUpdate(c, oldONT, ont)

	c.JSON(http.StatusOK, ToONTResponse(ont))
}

// ontUpdates turns the supplied fields into the columns to write.
func ontUpdates(req UpdateONTRequest) map[string]interface{} {
	updates := make(map[string]interface{})
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	return updates
}

func (h *ONTHandler) logONTUpdate(c *gin.Context, before, after *models.ONT) {
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"update",
		"ont",
		after.ID,
		map[string]interface{}{
			"description": before.Description,
			"status":      before.Status,
		},
		map[string]interface{}{
			"description": after.Description,
			"status":      after.Status,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)
}

// Delete handles DELETE /api/v1/onts/:id
func (h *ONTHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	// Get ONT for audit before deleting
	ont, err := h.ontService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: err.Error(),
		})
		return
	}

	if c.Query("remove_from_olt") == "true" && !h.clearFromOLT(c, id) {
		return
	}

	if err := h.ontService.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DELETE_FAILED",
			Error: err.Error(),
		})
		return
	}

	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"delete",
		"ont",
		id,
		map[string]interface{}{
			"serial_number": ont.SerialNumber,
			"olt_id":        ont.OLTID,
		},
		nil,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusOK, gin.H{"message": "ONT deleted successfully"})
}

// clearFromOLT removes the ONU from the chassis before its rows go. Deleting
// TikMan's rows first would leave an ONU configured on the OLT that nothing
// tracks any more, and the next discovery poll would simply register it again.
//
// It answers the request itself when it fails, so false means the delete must
// not proceed.
func (h *ONTHandler) clearFromOLT(c *gin.Context, id uuid.UUID) bool {
	if h.removalService == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Code:  "OLT_REMOVAL_UNAVAILABLE",
			Error: "This deployment has no CLI access configured for OLT removal",
		})
		return false
	}
	if err := h.removalService.RemoveFromOLT(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{
			Code:  "OLT_REMOVAL_FAILED",
			Error: err.Error(),
		})
		return false
	}
	return true
}
