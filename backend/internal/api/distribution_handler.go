package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
)

// DistributionHandler serves the passive plant: cabinets, the ports feeding
// them, and the distribution boxes a subscriber's drop lands in.
type DistributionHandler struct {
	service *services.DistributionService
}

// NewDistributionHandler constructs a DistributionHandler.
func NewDistributionHandler(service *services.DistributionService) *DistributionHandler {
	return &DistributionHandler{service: service}
}

// badRequest answers a validation failure with the operator's own sentence.
func badRequest(c *gin.Context, err error, code string) bool {
	if !errors.Is(err, services.ErrValidation) {
		return false
	}
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error: strings.TrimPrefix(err.Error(), services.ErrValidation.Error()+": "),
		Code:  code,
	})
	return true
}

// ListODCs answers with every cabinet and how much hangs off it.
func (h *DistributionHandler) ListODCs(c *gin.Context) {
	rows, err := h.service.ListODCs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODC_LIST_FAILED",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// CreateODC records a cabinet.
func (h *DistributionHandler) CreateODC(c *gin.Context) {
	var req CreateODCRequest
	if !bindJSON(c, &req) {
		return
	}
	siteID, err := uuid.Parse(req.SiteID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid site ID format", Code: "INVALID_SITE_ID",
		})
		return
	}

	feeds, err := odcFeedInputs(req.Feeds)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: err.Error(), Code: "INVALID_OLT_ID",
		})
		return
	}

	odc, err := h.service.CreateODCWithFeeds(services.ODCInput{
		SiteID: siteID, Code: req.Code,
		Latitude: req.Latitude, Longitude: req.Longitude,
		Address: req.Address, Notes: req.Notes,
	}, feeds)
	if err != nil {
		if badRequest(c, err, "INVALID_ODC") {
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODC_CREATE_FAILED",
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": odc})
}

// odcFeedInputs turns the feeds named in a create request into service inputs.
func odcFeedInputs(requests []CreateODCFeedRequest) ([]services.ODCFeedInput, error) {
	feeds := make([]services.ODCFeedInput, 0, len(requests))
	for _, req := range requests {
		oltID, err := uuid.Parse(req.OLTID)
		if err != nil {
			return nil, fmt.Errorf("invalid OLT ID format")
		}
		feeds = append(feeds, services.ODCFeedInput{
			OLTID: oltID, Slot: req.Slot, PortID: req.PortID,
			SplitterOutputs: req.SplitterOutputs, Notes: req.Notes,
		})
	}
	return feeds, nil
}

// AddODCFeed records one PON port supplying a cabinet.
func (h *DistributionHandler) AddODCFeed(c *gin.Context) {
	odcID, ok := pathUUID(c, "id", "INVALID_ODC_ID")
	if !ok {
		return
	}
	var req CreateODCFeedRequest
	if !bindJSON(c, &req) {
		return
	}
	oltID, err := uuid.Parse(req.OLTID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid OLT ID format", Code: "INVALID_OLT_ID",
		})
		return
	}

	feed, err := h.service.AddODCFeed(services.ODCFeedInput{
		ODCID: odcID, OLTID: oltID, Slot: req.Slot, PortID: req.PortID,
		SplitterOutputs: req.SplitterOutputs, Notes: req.Notes,
	})
	if err != nil {
		if badRequest(c, err, "INVALID_ODC_FEED") {
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODC_FEED_FAILED",
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": feed})
}

// ListODPs answers with every distribution box and the ports taken on it.
func (h *DistributionHandler) ListODPs(c *gin.Context) {
	rows, err := h.service.ListODPs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODP_LIST_FAILED",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// CreateODP records a distribution box under exactly one parent.
func (h *DistributionHandler) CreateODP(c *gin.Context) {
	var req CreateODPRequest
	if !bindJSON(c, &req) {
		return
	}
	parent, err := req.parent()
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: err.Error(), Code: "INVALID_ODP_PARENT",
		})
		return
	}

	parent.Code = req.Code
	parent.PortCount = req.PortCount
	parent.Latitude = req.Latitude
	parent.Longitude = req.Longitude
	parent.Address = req.Address
	parent.Notes = req.Notes

	odp, err := h.service.CreateODP(parent)
	if err != nil {
		if badRequest(c, err, "INVALID_ODP") {
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODP_CREATE_FAILED",
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": odp})
}

// SubscribersOnODP lists who is on which port of one box.
func (h *DistributionHandler) SubscribersOnODP(c *gin.Context) {
	odpID, ok := pathUUID(c, "id", "INVALID_ODP_ID")
	if !ok {
		return
	}
	onts, err := h.service.SubscribersOn(odpID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODP_SUBSCRIBERS_FAILED",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": onts})
}

// AssignONT lands a subscriber's drop on a port of a distribution box.
func (h *DistributionHandler) AssignONT(c *gin.Context) {
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

	if err := h.service.AssignONT(ontID, odpID, req.Port); err != nil {
		if badRequest(c, err, "INVALID_ODP_ASSIGNMENT") {
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODP_ASSIGN_FAILED",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Assigned"})
}

// UnassignONT takes a subscriber off its port, freeing it for someone else.
func (h *DistributionHandler) UnassignONT(c *gin.Context) {
	ontID, ok := pathUUID(c, "id", "INVALID_ONT_ID")
	if !ok {
		return
	}
	if err := h.service.UnassignONT(ontID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "ODP_UNASSIGN_FAILED",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Unassigned"})
}
