package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PreviewRemoval handles GET /api/v1/onts/:id/removal/preview. It returns the
// commands a removal would send without sending them, so the operator sees
// what will reach a live OLT before agreeing to it.
func (h *ONTHandler) PreviewRemoval(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}
	if h.removalService == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Code:  "OLT_REMOVAL_UNAVAILABLE",
			Error: "This deployment has no CLI access configured for OLT removal",
		})
		return
	}

	commands, err := h.removalService.PreviewRemoval(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "PREVIEW_FAILED",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ont_id": id, "commands": commands})
}
