package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PonHealth draws the fault down to the port it sits on.
//
// The subscriber ranking answers who is failing. Forty-one subscribers churning
// on one PON are one fault at the port, not forty-one in homes, and this is the
// view that says so.
func (h *OLTHandler) PonHealth(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code: "INVALID_ID", Error: "Invalid OLT ID format",
		})
		return
	}

	window, _ := troubledQuery(c)

	health, err := h.ontService.PonHealthFor(oltID, window)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code: "OLT_NOT_FOUND", Error: "OLT not found",
		})
	case err != nil:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code: "PON_HEALTH_FAILED", Error: err.Error(),
		})
	default:
		c.JSON(http.StatusOK, gin.H{"data": health, "hours": int(window.Hours())})
	}
}
