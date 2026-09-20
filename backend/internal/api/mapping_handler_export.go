package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// kmzContentType is the IANA-registered type for a KMZ archive.
const kmzContentType = "application/vnd.google-earth.kmz"

// kmzWarningHeader carries the one-sentence Indonesian warning when the
// export left cables out for a missing endpoint - absent entirely on a
// clean export, not present-but-empty. corsMiddleware exposes it explicitly,
// since a custom response header is otherwise invisible to cross-origin JS.
const kmzWarningHeader = "X-Kmz-Warning"

// ExportKMZ answers the whole map as a KMZ: a zip containing one doc.kml,
// which is what Google Earth expects to open directly. Open to any signed-in
// user, the same as ListNodes/ListEdges - this is a read, not a change.
func (h *MappingHandler) ExportKMZ(c *gin.Context) {
	kmz, warning, err := h.mapping.ExportKMZ()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "MAPPING_FAILED"})
		return
	}
	if warning != "" {
		c.Header(kmzWarningHeader, warning)
	}
	c.Header("Content-Disposition", `attachment; filename="peta-jaringan.kmz"`)
	c.Data(http.StatusOK, kmzContentType, kmz)
}
