package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// ONTHandler handles ONT HTTP requests
type ONTHandler struct {
	ontService     *services.ONTService
	metricsService *services.MetricsService
	auditService   *services.AuditService
	removalService *services.ZTEONURemovalService
}

// NewONTHandler creates a new ONT handler
func NewONTHandler(ontService *services.ONTService, metricsService *services.MetricsService, auditService *services.AuditService, removalService *services.ZTEONURemovalService) *ONTHandler {
	return &ONTHandler{
		ontService:     ontService,
		metricsService: metricsService,
		auditService:   auditService,
		removalService: removalService,
	}
}

// List handles GET /api/v1/onts
// maxONTPageSize bounds one page of ONTs. The old cap of 500 sat below a single
// populated chassis — Cariu carries 651 — so a caller asking for 1000 was
// silently answered with 500, and every page that counted the rows it received
// understated the network without saying so. A full ZTE C320 fits inside this.
//
// It is a ceiling, not a page size to aim for: the overview reads
// /dashboard/stats instead, because counting rows in the browser is what made a
// cap load-bearing in the first place.
const maxONTPageSize = 5000

// optionalInt reads a query parameter that narrows the list, returning nil when
// it is absent or not a number.
func optionalInt(raw string) *int {
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func (h *ONTHandler) List(c *gin.Context) {
	filter, ok := ontListFilter(c)
	if !ok {
		return
	}

	onts, total, err := h.ontService.ListFiltered(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "LIST_FAILED",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   h.decorateONTs(onts),
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// ontListFilter reads the query into a filter, answering the request itself
// and returning false when a parameter is malformed in a way that would
// silently change which rows come back.
func ontListFilter(c *gin.Context) (services.ONTListFilter, bool) {
	var filter services.ONTListFilter

	if oltIDStr := c.Query("olt_id"); oltIDStr != "" {
		id, err := uuid.Parse(oltIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:    "INVALID_OLT_ID",
				Error:   "Invalid OLT ID format",
				Details: map[string]string{"olt_id": oltIDStr},
			})
			return filter, false
		}
		filter.OLTID = &id
	}

	if statusStr := c.Query("status"); statusStr != "" {
		status := models.ONTStatus(statusStr)
		filter.Status = &status
	}

	start, ok := queryTime(c, "start_time")
	if !ok {
		return filter, false
	}
	end, ok := queryTime(c, "end_time")
	if !ok {
		return filter, false
	}
	filter.StartTime, filter.EndTime = start, end

	// A card and port narrow a position. Anything unparseable is left unset
	// rather than rejected: a stray query parameter should widen the answer, not
	// fail the page an operator is looking at.
	filter.Slot = optionalInt(c.Query("slot"))
	filter.PortID = optionalInt(c.Query("port_id"))
	filter.Search = strings.TrimSpace(c.Query("search"))
	filter.Limit, filter.Offset = pageBounds(c)

	return filter, true
}

// queryTime reads an RFC3339 parameter. Unlike the position filters a bad
// timestamp is refused, because a range the operator asked for and did not get
// would read as missing history.
func queryTime(c *gin.Context, name string) (*time.Time, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_TIME_RANGE",
			Error: fmt.Sprintf("Invalid %s format: %v", name, err),
		})
		return nil, false
	}
	return &value, true
}

// pageBounds caps a page at maxONTPageSize, and reads a limit below 1 as a
// request for everything rather than for nothing.
func pageBounds(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > maxONTPageSize {
		limit = maxONTPageSize
	}
	return limit, offset
}

// decorateONTs attaches the OLT name and the latest reading to each row, both
// fetched for the whole page at once: a per-row lookup is what made a 5000-ONT
// list slow enough to notice.
func (h *ONTHandler) decorateONTs(onts []models.ONT) []ONTResponse {
	responses := make([]ONTResponse, len(onts))
	if len(onts) == 0 {
		return responses
	}

	ontIDs := make([]uuid.UUID, len(onts))
	oltIDs := make([]uuid.UUID, 0)
	seenOLT := make(map[uuid.UUID]bool)
	for i, ont := range onts {
		ontIDs[i] = ont.ID
		if !seenOLT[ont.OLTID] {
			oltIDs = append(oltIDs, ont.OLTID)
			seenOLT[ont.OLTID] = true
		}
	}

	oltNames := make(map[uuid.UUID]string)
	if olts, err := h.ontService.GetONTOlts(oltIDs); err == nil {
		for _, olt := range olts {
			oltNames[olt.ID] = olt.Name
		}
	}
	metricsMap, _ := h.metricsService.GetLatestMetricsBatch(ontIDs)

	for i, ont := range onts {
		resp := ToONTResponseWithMetrics(&ont, metricsMap[ont.ID])
		resp.OLTName = oltNames[ont.OLTID]
		responses[i] = resp
	}
	return responses
}

func (h *ONTHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	ont, err := h.ontService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: err.Error(),
		})
		return
	}

	metrics, _ := h.metricsService.GetLatestMetrics(ont.ID)

	c.JSON(http.StatusOK, ToONTResponseWithMetrics(ont, metrics))
}
