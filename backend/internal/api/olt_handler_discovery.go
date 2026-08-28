package api

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/middleware"
)

// DiscoverOLTTopology handles POST /api/v1/olts/:id/topology discovery
func (h *OLTHandler) DiscoverOLTTopology(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	// Get OLT details using the existing service
	olt, err := h.service.GetByID(oltID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: "OLT not found",
		})
		return
	}

	// Decrypt SNMP community if needed
	snmpCommunity := olt.SNMPCommunity
	if snmpCommunity == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "CONFIG_ERROR",
			Error: "SNMP community not configured for this OLT",
		})
		return
	}

	// Perform topology discovery
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "UNSUPPORTED_MODEL",
			Error: err.Error(),
		})
		return
	}

	topology, err := connectivity.DiscoverOLTTopology(driver, olt.IPAddress, snmpCommunity, olt.SNMPPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":   oltID,
		"topology": topology,
	})
}

// DiscoverONTs handles POST /api/v1/olts/:id/discover
func (h *OLTHandler) DiscoverONTs(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	// Get OLT details using the existing service
	olt, err := h.service.GetByID(oltID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: "OLT not found",
		})
		return
	}

	// Decrypt SNMP community if needed
	snmpCommunity := olt.SNMPCommunity
	if snmpCommunity == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "CONFIG_ERROR",
			Error: "SNMP community not configured for this OLT",
		})
		return
	}

	// Use legacy flat discovery (deprecated but kept for backwards compat)
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "UNSUPPORTED_MODEL",
			Error: err.Error(),
		})
		return
	}

	discovered, err := connectivity.DiscoverONTs(driver, olt.IPAddress, snmpCommunity, olt.SNMPPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Convert to response format
	results := make([]gin.H, len(discovered))
	for i, ont := range discovered {
		results[i] = gin.H{
			"port_id":       ont.PortID,
			"ont_id":        ont.ONTID,
			"serial_number": ont.SerialNumber,
			"run_state":     ont.RunState,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"discovered": len(discovered),
		"onts":       results,
	})
}

// DiscoverAndRegisterONTs handles POST /api/v1/olts/:id/discover-and-register
func (h *OLTHandler) DiscoverAndRegisterONTs(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	// Perform discovery
	discovered, err := h.service.DiscoverONTs(oltID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Bulk register
	result := h.ontService.BulkRegisterFromDiscovery(oltID, discovered)

	// Audit log
	if h.auditService != nil && result.Registered > 0 {
		actorID, _ := middleware.GetUserID(c)
		_ = h.auditService.Log(
			actorID,
			"bulk_create",
			"ont",
			oltID,
			map[string]interface{}{"registered": result.Registered},
			nil,
			fmt.Sprintf("Registered %d ONTs from discovery", result.Registered),
			"",
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"discovered": len(discovered),
		"registered": result.Registered,
		"skipped":    result.Skipped,
		"errors":     result.Errors,
	})
}

// GetCachedTopology handles GET /api/v1/olts/:id/topology/cached - returns topology from database
func (h *OLTHandler) GetCachedTopology(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	onts, err := h.ontService.ListONTSummariesForOLT(oltID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "QUERY_FAILED",
			Error: "Failed to query ONTs from database",
		})
		return
	}

	// Grouped by the ONT's own card. This used to hardcode slot 1 on the
	// grounds that the model had no slot; it has one, and it is populated, so
	// the filter was offering "Card 1" for ONUs sitting on card 3.
	slotMap := make(map[int]map[int][]map[string]interface{})

	// Seeded with every fitted card, so one carrying no ONU still appears. A
	// card cannot be inferred from where ONUs live.
	cards, err := h.service.ListCards(oltID)
	if err != nil {
		cards = nil
	}
	for _, card := range cards {
		if slotMap[card.Slot] == nil {
			slotMap[card.Slot] = make(map[int][]map[string]interface{})
		}
	}

	for _, ont := range onts {
		if ont.Slot == nil {
			continue
		}
		slot := *ont.Slot
		if slotMap[slot] == nil {
			slotMap[slot] = make(map[int][]map[string]interface{})
		}

		ontData := map[string]interface{}{
			"ont_id":        ont.ONTID,
			"port_id":       ont.PortID,
			"serial_number": ont.SerialNumber,
			"status":        ont.Status,
			"name":          ont.Name,
			"description":   ont.Description,
			"run_state":     mapStatusToRunState(ont.Status),
		}

		slotMap[slot][ont.PortID] = append(slotMap[slot][ont.PortID], ontData)
	}

	// Sorted, because a map's order changes between requests and the card and
	// PON dropdowns are built straight from this. Empty lists rather than nil:
	// a fitted card with no ONU must still answer with an array.
	slots := make([]int, 0, len(slotMap))
	for slot := range slotMap {
		slots = append(slots, slot)
	}
	sort.Ints(slots)

	topology := make([]map[string]interface{}, 0, len(slots))
	for _, slot := range slots {
		ports := slotMap[slot]
		portIDs := make([]int, 0, len(ports))
		for portID := range ports {
			portIDs = append(portIDs, portID)
		}
		sort.Ints(portIDs)

		portList := make([]map[string]interface{}, 0, len(portIDs))
		for _, portID := range portIDs {
			portList = append(portList, map[string]interface{}{
				"port_id": portID,
				"onts":    ports[portID],
			})
		}

		topology = append(topology, map[string]interface{}{
			"slot":  slot,
			"ports": portList,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"topology": topology,
		"source":   "database",
	})
}

// mapStatusToRunState converts ONT status to run_state integer
func mapStatusToRunState(status string) int {
	switch status {
	case "online":
		return 3
	case "offline":
		return 6
	case "los":
		return 1
	case "dying_gasp":
		return 4
	default:
		return 0
	}
}
