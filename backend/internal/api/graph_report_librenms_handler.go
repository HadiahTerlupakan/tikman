package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/librenms"
)

// previewWindow matches what the daily report sends.
const previewWindow = 24 * time.Hour

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

// LibreNMSGraph answers the graph a port would be reported with now.
func (h *GraphReportHandler) LibreNMSGraph(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Query("device_id"))
	ifName := strings.TrimSpace(c.Query("if_name"))
	if err != nil || deviceID <= 0 || ifName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "device_id dan if_name wajib diisi", Code: "INVALID_GRAPH_REQUEST",
		})
		return
	}
	client, ok := h.libreNMS(c)
	if !ok {
		return
	}
	now := time.Now()
	image, err := client.PortGraph(c.Request.Context(), deviceID, ifName, now.Add(-previewWindow), now)
	if err != nil {
		upstreamError(c, err)
		return
	}
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
