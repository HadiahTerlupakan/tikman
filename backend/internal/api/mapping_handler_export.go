package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// kmzContentType is the IANA-registered type for a KMZ archive.
const kmzContentType = "application/vnd.google-earth.kmz"

// ExportKMZ answers the whole map as a KMZ: a zip containing one doc.kml,
// which is what Google Earth expects to open directly. Open to any signed-in
// user, the same as ListNodes/ListEdges - this is a read, not a change.
func (h *MappingHandler) ExportKMZ(c *gin.Context) {
	kmz, err := h.mapping.ExportKMZ()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "MAPPING_FAILED"})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="peta-jaringan.kmz"`)
	c.Data(http.StatusOK, kmzContentType, kmz)
}
