package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/services"
)

// maxImportUploadBytes bounds the raw .kmz upload itself, before it is ever
// unzipped - the same layering cs_handler_messages.go's maxUploadBytes uses
// for an outgoing attachment. mapping_kml_import_parse.go's own
// maxKMLDecompressedBytes is a second, independent cap on what comes out of
// the zip, since a small upload can still decompress to something enormous.
const maxImportUploadBytes = 25 << 20

// PreviewImport parses an uploaded KMZ and reports what it found. Nothing is
// written - see MappingService.PreviewImport. Gated to the same editor role
// as every other write-adjacent mapping route (router.go); a preview reveals
// how a file's ids collide with the live map, which is not something a
// viewer needs.
func (h *MappingHandler) PreviewImport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportUploadBytes)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		refuseImportUpload(c, err)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "gagal membuka berkas", Code: "IMPORT_FILE_INVALID"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "gagal membaca berkas", Code: "IMPORT_FILE_INVALID"})
		return
	}

	preview, err := h.mapping.PreviewImport(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "IMPORT_PARSE_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": preview})
}

// refuseImportUpload mirrors refuseUpload in cs_handler_messages.go for this
// endpoint's own two ways a multipart body yields no file.
func refuseImportUpload(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("berkas melebihi batas %d MB", maxImportUploadBytes>>20),
			Code:  "IMPORT_FILE_TOO_LARGE",
		})
		return
	}
	c.JSON(http.StatusBadRequest, ErrorResponse{Error: "berkas KMZ wajib diisi", Code: "IMPORT_FILE_REQUIRED"})
}

// importCommitRequest is decoded straight into the service's own row types:
// the preview response and the commit request are the same shape by design
// (upload, edit in place, send back), so a separate near-identical DTO would
// only duplicate ImportedNode/ImportedEdge's fields for no second caller.
type importCommitRequest struct {
	Nodes []services.ImportedNode `json:"nodes"`
	Edges []services.ImportedEdge `json:"edges"`
}

// CommitImport writes exactly what the person confirmed in the preview
// screen, in one transaction - see MappingService.CommitImport for what is
// re-checked before anything is written.
func (h *MappingHandler) CommitImport(c *gin.Context) {
	var req importCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_IMPORT"})
		return
	}
	result, err := h.mapping.CommitImport(req.Nodes, req.Edges)
	if err != nil {
		mappingError(c, err, "IMPORT_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
