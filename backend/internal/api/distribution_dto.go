package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// bindJSON reads a request body, answering the caller and reporting false when
// it cannot.
func bindJSON(c *gin.Context, target interface{}) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return false
	}
	return true
}

// pathUUID reads a UUID out of the path, answering the caller and reporting
// false when it is not one.
func pathUUID(c *gin.Context, param, code string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ID format", Code: code,
		})
		return uuid.Nil, false
	}
	return id, true
}

// CreateODCRequest records a cabinet.
type CreateODCRequest struct {
	SiteID string `json:"site_id" binding:"required,uuid"`
	// The code is the cabinet's identity, so it is what the form must carry.
	Code      string   `json:"code" binding:"required,min=1,max=64"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Address   string   `json:"address"`
	Notes     string   `json:"notes"`
	// Feeds arrive with the cabinet so the two are saved together: a cabinet
	// kept without the feed that was refused would stand fed by nothing.
	Feeds []CreateODCFeedRequest `json:"feeds"`
}

// CreateODCFeedRequest records one PON port supplying a cabinet.
type CreateODCFeedRequest struct {
	OLTID           string `json:"olt_id" binding:"required,uuid"`
	Slot            int    `json:"slot"`
	PortID          int    `json:"port_id"`
	SplitterOutputs int    `json:"splitter_outputs" binding:"required,min=1"`
	Notes           string `json:"notes"`
}

// CreateODPRequest records a distribution box. Exactly one parent is named:
// odc_id, or the olt_id/slot/port_id triple.
type CreateODPRequest struct {
	Code      string   `json:"code" binding:"required,min=1,max=64"`
	PortCount int      `json:"port_count" binding:"required,min=1"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Address   string   `json:"address"`
	Notes     string   `json:"notes"`

	ODCID  string `json:"odc_id" binding:"omitempty,uuid"`
	OLTID  string `json:"olt_id" binding:"omitempty,uuid"`
	Slot   *int   `json:"slot"`
	PortID *int   `json:"port_id"`
}

// parent turns the request's two ways of naming a parent into the one the
// service takes. The service and the database both refuse a box with two
// parents or none; this only translates what was asked.
func (r CreateODPRequest) parent() (services.ODPInput, error) {
	in := services.ODPInput{}

	if r.ODCID != "" {
		id, err := uuid.Parse(r.ODCID)
		if err != nil {
			return in, fmt.Errorf("invalid ODC ID format")
		}
		in.ODCID = &id
	}
	if r.OLTID != "" {
		id, err := uuid.Parse(r.OLTID)
		if err != nil {
			return in, fmt.Errorf("invalid OLT ID format")
		}
		in.OLTID = &id
	}
	in.Slot = r.Slot
	in.PortID = r.PortID
	return in, nil
}

// AssignONTToODPRequest lands a subscriber on a port.
type AssignONTToODPRequest struct {
	ODPID string `json:"odp_id" binding:"required,uuid"`
	Port  int    `json:"port" binding:"required,min=1"`
}

// SetRouteRequest is the path traced for one cable, or an empty list to hand it
// back to the straight line the map draws on its own.
type SetRouteRequest struct {
	Route []models.RoutePoint `json:"route"`
}
