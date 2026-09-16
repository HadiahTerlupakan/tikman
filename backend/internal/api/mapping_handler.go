package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/services"
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
func mappingError(c *gin.Context, err error, notFoundCode string) {
	switch {
	case errors.Is(err, services.ErrNodeExists), errors.Is(err, services.ErrEdgeExists),
		errors.Is(err, services.ErrSlotsFull):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "MAPPING_CONFLICT"})
	default:
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error(), Code: notFoundCode})
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
