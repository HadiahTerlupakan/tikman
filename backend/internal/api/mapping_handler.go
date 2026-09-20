package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// MappingHandler serves the network map.
type MappingHandler struct {
	mapping *services.MappingService
}

func NewMappingHandler(mapping *services.MappingService) *MappingHandler {
	return &MappingHandler{mapping: mapping}
}

// mappingError turns a service error into the answer a technician sees. A full
// box and a repeated id are both 409: the request was understood and refused.
// Each sentinel still gets its own code — a full ODC and a duplicate cable look
// nothing alike to an operator, and collapsing them to one code left the
// frontend unable to tell "free a slot" from "this already exists". Only a
// wrapped gorm.ErrRecordNotFound is a 404 — CreateNode has no legitimate
// not-found path, so anything else here is a real failure, and reporting it as
// a missing node would send someone hunting for the wrong thing.
func mappingError(c *gin.Context, err error, notFoundCode string) {
	switch {
	case errors.Is(err, services.ErrNodeExists):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "NODE_EXISTS"})
	case errors.Is(err, services.ErrEdgeExists):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "EDGE_EXISTS"})
	case errors.Is(err, services.ErrSlotsFull):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "SLOTS_FULL"})
	case errors.Is(err, services.ErrNodeInUse):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "NODE_IN_USE"})
	case errors.Is(err, services.ErrNodeMirrorsOLT):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "NODE_MIRRORS_OLT"})
	case errors.Is(err, services.ErrValidation):
		// Reachable today only from UpdateNode's coordinate check on an
		// OLT-backed node (mapping_nodes.go) - same code as
		// olt_handler_crud_update.go uses for the OLT menu's own
		// latitude/longitude fields, since it is the same rule.
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: strings.TrimPrefix(err.Error(), services.ErrValidation.Error()+": "),
			Code:  "INVALID_COORDINATES",
		})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error(), Code: notFoundCode})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "MAPPING_FAILED"})
	}
}

func (h *MappingHandler) ListNodes(c *gin.Context) {
	nodes, err := h.mapping.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "MAPPING_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": nodes})
}

func (h *MappingHandler) CreateNode(c *gin.Context) {
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_NODE"})
		return
	}
	node, err := h.mapping.CreateNode(req.toModel())
	if err != nil {
		mappingError(c, err, "NODE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": node})
}

func (h *MappingHandler) UpdateNode(c *gin.Context) {
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_NODE"})
		return
	}
	node, err := h.mapping.UpdateNode(c.Param("node_id"), req.toModel())
	if err != nil {
		mappingError(c, err, "NODE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": node})
}

func (h *MappingHandler) DeleteNode(c *gin.Context) {
	if err := h.mapping.DeleteNode(c.Param("node_id")); err != nil {
		mappingError(c, err, "NODE_NOT_FOUND")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MappingHandler) ListEdges(c *gin.Context) {
	edges, err := h.mapping.ListEdges()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "MAPPING_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": edges})
}

func (h *MappingHandler) CreateEdge(c *gin.Context) {
	var req edgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_EDGE"})
		return
	}
	edge, err := h.mapping.CreateEdge(req.toModel())
	if err != nil {
		mappingError(c, err, "EDGE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": edge})
}

func (h *MappingHandler) UpdateEdge(c *gin.Context) {
	var req edgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_EDGE"})
		return
	}
	edge, err := h.mapping.UpdateEdge(c.Param("edge_id"), req.toModel())
	if err != nil {
		mappingError(c, err, "EDGE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": edge})
}

func (h *MappingHandler) DeleteEdge(c *gin.Context) {
	if err := h.mapping.DeleteEdge(c.Param("edge_id")); err != nil {
		mappingError(c, err, "EDGE_NOT_FOUND")
		return
	}
	c.Status(http.StatusNoContent)
}
