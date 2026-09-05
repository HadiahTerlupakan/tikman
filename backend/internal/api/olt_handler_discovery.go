package api

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// oltForDiscovery resolves the OLT a discovery route names and the driver for
// its model. It answers the request itself on every failure, so a false return
// means the caller owes the client nothing further.
func (h *OLTHandler) oltForDiscovery(c *gin.Context) (*models.OLT, connectivity.Driver, bool) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return nil, nil, false
	}

	olt, err := h.service.GetByID(oltID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: "OLT not found",
		})
		return nil, nil, false
	}

	if olt.SNMPCommunity == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "CONFIG_ERROR",
			Error: "SNMP community not configured for this OLT",
		})
		return nil, nil, false
	}

	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "UNSUPPORTED_MODEL",
			Error: err.Error(),
		})
		return nil, nil, false
	}

	return olt, driver, true
}

// DiscoverOLTTopology handles POST /api/v1/olts/:id/topology discovery
func (h *OLTHandler) DiscoverOLTTopology(c *gin.Context) {
	olt, driver, ok := h.oltForDiscovery(c)
	if !ok {
		return
	}

	topology, err := connectivity.DiscoverOLTTopology(driver, olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":   olt.ID,
		"topology": topology,
	})
}

// DiscoverONTs handles POST /api/v1/olts/:id/discover with the flat walk kept
// for backwards compatibility.
func (h *OLTHandler) DiscoverONTs(c *gin.Context) {
	olt, driver, ok := h.oltForDiscovery(c)
	if !ok {
		return
	}

	discovered, err := connectivity.DiscoverONTs(driver, olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

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
		"olt_id":     olt.ID,
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

	discovered, err := h.service.DiscoverONTs(oltID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	result := h.ontService.BulkRegisterFromDiscovery(oltID, discovered)
	h.logBulkRegistration(c, oltID, result.Registered)

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"discovered": len(discovered),
		"registered": result.Registered,
		"skipped":    result.Skipped,
		"errors":     result.Errors,
	})
}

func (h *OLTHandler) logBulkRegistration(c *gin.Context, oltID uuid.UUID, registered int) {
	if h.auditService == nil || registered == 0 {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"bulk_create",
		"ont",
		oltID,
		map[string]interface{}{"registered": registered},
		nil,
		fmt.Sprintf("Registered %d ONTs from discovery", registered),
		"",
	)
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

	// A card cannot be inferred from where ONUs live, so the fitted cards are
	// read separately and a card carrying none still appears.
	cards, err := h.service.ListCards(oltID)
	if err != nil {
		cards = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"topology": renderTopology(groupONTsByCard(cards, onts)),
		"source":   "database",
	})
}

// groupONTsByCard indexes the ONTs by the card and PON port they sit on.
//
// Grouping used to hardcode slot 1 on the grounds that the model had no slot;
// it has one, and it is populated, so the filter was offering "Card 1" for
// ONUs sitting on card 3.
func groupONTsByCard(cards []connectivity.ZTECard, onts []services.ONTSummary) map[int]map[int][]map[string]interface{} {
	byCard := make(map[int]map[int][]map[string]interface{})
	for _, card := range cards {
		if byCard[card.Slot] == nil {
			byCard[card.Slot] = make(map[int][]map[string]interface{})
		}
	}

	for _, ont := range onts {
		if ont.Slot == nil {
			continue
		}
		slot := *ont.Slot
		if byCard[slot] == nil {
			byCard[slot] = make(map[int][]map[string]interface{})
		}
		byCard[slot][ont.PortID] = append(byCard[slot][ont.PortID], map[string]interface{}{
			"ont_id":        ont.ONTID,
			"port_id":       ont.PortID,
			"serial_number": ont.SerialNumber,
			"status":        ont.Status,
			"name":          ont.Name,
			"description":   ont.Description,
			"run_state":     mapStatusToRunState(ont.Status),
		})
	}
	return byCard
}

// renderTopology sorts the grouping into the arrays the card and PON dropdowns
// are built from. Sorted, because a map's order changes between requests; and
// empty lists rather than nil, because a fitted card with no ONU must still
// answer with an array.
func renderTopology(byCard map[int]map[int][]map[string]interface{}) []map[string]interface{} {
	slots := make([]int, 0, len(byCard))
	for slot := range byCard {
		slots = append(slots, slot)
	}
	sort.Ints(slots)

	topology := make([]map[string]interface{}, 0, len(slots))
	for _, slot := range slots {
		ports := byCard[slot]
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
	return topology
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
