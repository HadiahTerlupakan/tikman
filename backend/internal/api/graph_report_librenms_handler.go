package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/librenms"
	"github.com/tikman/olt-provisioning/internal/services"
)

// LibreNMSDevice is a device as the port picker lists it.
type LibreNMSDevice struct {
	DeviceID int    `json:"device_id"`
	Hostname string `json:"hostname"`
	SysName  string `json:"sys_name"`
}

// LibreNMSPort is a port as the port picker lists it.
type LibreNMSPort struct {
	PortID     int    `json:"port_id"`
	IfName     string `json:"if_name"`
	IfAlias    string `json:"if_alias"`
	OperStatus string `json:"oper_status"`
}

// LibreNMSDevices lists LibreNMS's devices. The page also uses it to test the
// configured address and token.
func (h *GraphReportHandler) LibreNMSDevices(c *gin.Context) {
	client, ok := h.libreNMS(c)
	if !ok {
		return
	}
	devices, err := client.Devices(c.Request.Context())
	if err != nil {
		upstreamError(c, err)
		return
	}
	out := make([]LibreNMSDevice, len(devices))
	for i, device := range devices {
		out[i] = LibreNMSDevice{DeviceID: device.DeviceID, Hostname: device.Hostname, SysName: device.SysName}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// LibreNMSPorts lists one device's ports.
func (h *GraphReportHandler) LibreNMSPorts(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Param("deviceId"))
	if err != nil || deviceID <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "device_id tidak valid", Code: "INVALID_DEVICE_ID"})
		return
	}
	client, ok := h.libreNMS(c)
	if !ok {
		return
	}
	ports, err := client.Ports(c.Request.Context(), deviceID)
	if err != nil {
		upstreamError(c, err)
		return
	}
	out := make([]LibreNMSPort, len(ports))
	for i, port := range ports {
		out[i] = LibreNMSPort{PortID: port.PortID, IfName: port.IfName, IfAlias: port.IfAlias, OperStatus: port.IfOperStatus}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// LibreNMSGraph answers one port's section of the report image as it would
// be sent now, titled with label, or with the ifName when none is given yet.
func (h *GraphReportHandler) LibreNMSGraph(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Query("device_id"))
	ifName := strings.TrimSpace(c.Query("if_name"))
	if err != nil || deviceID <= 0 || ifName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "device_id dan if_name wajib diisi", Code: "INVALID_GRAPH_REQUEST",
		})
		return
	}
	label := strings.TrimSpace(c.Query("label"))
	if label == "" {
		label = ifName
	}
	image, err := h.sender.PortPreview(c.Request.Context(), deviceID, ifName, label)
	if err != nil {
		graphReportError(c, err, "LIBRENMS_FAILED")
		return
	}
	pngResponse(c, image)
}

// GraphReportPreviewRequest is a report's form as it stands, before it is
// saved; the group is not needed to draw it.
type GraphReportPreviewRequest struct {
	CustomerName string                   `json:"customer_name"`
	Message      string                   `json:"message"`
	Ports        []GraphReportPortRequest `json:"ports"`
}

// Preview answers the exact image a report would be sent as now.
func (h *GraphReportHandler) Preview(c *gin.Context) {
	var req GraphReportPreviewRequest
	if !bindJSON(c, &req) {
		return
	}
	image, err := h.sender.Preview(c.Request.Context(), services.GraphReportInput{
		CustomerName: req.CustomerName, Message: req.Message, Ports: portInputs(req.Ports),
	})
	if err != nil {
		graphReportError(c, err, "GRAPH_REPORT_PREVIEW_FAILED")
		return
	}
	pngResponse(c, image)
}

func pngResponse(c *gin.Context, image []byte) {
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/png", image)
}

func (h *GraphReportHandler) libreNMS(c *gin.Context) (*librenms.Client, bool) {
	client, err := h.sender.LibreNMS()
	if err != nil {
		graphReportError(c, err, "LIBRENMS_UNAVAILABLE")
		return nil, false
	}
	return client, true
}

func upstreamError(c *gin.Context, err error) {
	c.JSON(http.StatusBadGateway, ErrorResponse{Error: err.Error(), Code: "LIBRENMS_FAILED"})
}
