package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssignONTToODPRequest lands a subscriber's drop on a port of a distribution
// box. The min=1 tag is a first line of defense; ONTService.AssignONTToODP
// still refuses it itself, since this DTO is not the only caller.
type AssignONTToODPRequest struct {
	ODPID string `json:"odp_id" binding:"required,uuid"`
	Port  int    `json:"port" binding:"required,min=1"`
}

// AssignOdp handles PUT /api/v1/onts/:id/odp
func (h *ONTHandler) AssignOdp(c *gin.Context) {
	ontID, ok := pathUUID(c, "id", "INVALID_ONT_ID")
	if !ok {
		return
	}
	var req AssignONTToODPRequest
	if !bindJSON(c, &req) {
		return
	}
	odpID, err := uuid.Parse(req.ODPID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ODP ID format", Code: "INVALID_ODP_ID",
		})
		return
	}

	if err := h.ontService.AssignONTToODP(ontID, odpID, req.Port); err != nil {
		if badRequest(c, err, "INVALID_ODP_ASSIGNMENT") {
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "ONT not found", Code: "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODP_ASSIGN_FAILED",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Assigned"})
}

// UnassignOdp handles DELETE /api/v1/onts/:id/odp
func (h *ONTHandler) UnassignOdp(c *gin.Context) {
	ontID, ok := pathUUID(c, "id", "INVALID_ONT_ID")
	if !ok {
		return
	}
	if err := h.ontService.UnassignONTFromODP(ontID); err != nil {
		if badRequest(c, err, "INVALID_ODP_UNASSIGNMENT") {
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "ONT not found", Code: "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODP_UNASSIGN_FAILED",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Unassigned"})
}
