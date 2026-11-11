package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tikman/olt-provisioning/internal/librenms"
	"github.com/tikman/olt-provisioning/internal/models"
)

// DefaultGraphReportMessage is the caption of a report whose admin left the
// message empty. The frontend shows the same text as the field's placeholder.
const DefaultGraphReportMessage = "Selamat pagi Rekan,\n\nBerikut kami lampirkan hasil monitoring trafik " +
	"24 jam terakhir untuk {pelanggan}.\n\nTerima kasih"

// customerPlaceholder is replaced by the report's customer name.
const customerPlaceholder = "{pelanggan}"

// maxGraphReportImageBytes matches the inbox's upload cap, the largest
// attachment the wa process is meant to send. Ten stacked graphs are far
// below it; anything near it means a graph was not what LibreNMS should send.
const maxGraphReportImageBytes = 16 << 20

// graphRangeLayout is how each graph's window is written above it.
const graphRangeLayout = "02/01/2006 15:04"

// GraphReportCaption is the text a report's image is sent with.
func GraphReportCaption(message, customer string) string {
	if strings.TrimSpace(message) == "" {
		message = DefaultGraphReportMessage
	}
	return strings.ReplaceAll(message, customerPlaceholder, customer)
}

// graphRange writes the window a graph covers, in WIB.
func graphRange(now time.Time) string {
	return now.Add(-graphWindow).In(wib).Format(graphRangeLayout) + " – " +
		now.In(wib).Format(graphRangeLayout) + " WIB"
}

// renderReport fetches every port's graph and stacks them into one image. It
// is all or nothing: one graph LibreNMS cannot draw fails the whole report.
func renderReport(ctx context.Context, client *librenms.Client, ports []models.GraphReportPort, now time.Time) ([]byte, error) {
	if len(ports) == 0 {
		return nil, fmt.Errorf("%w: laporan tidak punya port", ErrValidation)
	}
	span := graphRange(now)
	sections := make([]librenms.Section, len(ports))
	for i, port := range ports {
		graph, err := client.PortGraph(ctx, port.DeviceID, port.IfName, now.Add(-graphWindow), now)
		if err != nil {
			return nil, fmt.Errorf("%w: port %s (%s %s): %v", ErrGraphFetch, port.Label, port.DeviceName, port.IfName, err)
		}
		sections[i] = librenms.Section{Title: port.Label, Subtitle: span, Graph: graph}
	}
	image, err := librenms.Compose(sections)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGraphFetch, err)
	}
	if err := checkGraphReportImageSize(len(image)); err != nil {
		return nil, err
	}
	return image, nil
}

func checkGraphReportImageSize(size int) error {
	if size > maxGraphReportImageBytes {
		return fmt.Errorf("%w: gambar laporan %d byte, melebihi batas %d byte",
			ErrGraphFetch, size, maxGraphReportImageBytes)
	}
	return nil
}

// Preview renders the image a report with these ports would be sent as now.
// The group is not needed for it and is not checked.
func (s *GraphReportSender) Preview(ctx context.Context, in GraphReportInput) ([]byte, error) {
	if err := validateGraphReportFields(in); err != nil {
		return nil, err
	}
	client, err := s.LibreNMS()
	if err != nil {
		return nil, err
	}
	return renderReport(ctx, client, portRows(in.Ports), s.now())
}

// PortPreview renders one port's section of a report, titled label.
func (s *GraphReportSender) PortPreview(ctx context.Context, deviceID int, ifName, label string) ([]byte, error) {
	// The label comes straight from the query string here, unlike a saved
	// port's; an unbounded one makes fitting it to the band quadratic.
	if err := requireText(label, "label"); err != nil {
		return nil, err
	}
	client, err := s.LibreNMS()
	if err != nil {
		return nil, err
	}
	port := models.GraphReportPort{DeviceID: deviceID, IfName: ifName, Label: label}
	return renderReport(ctx, client, []models.GraphReportPort{port}, s.now())
}

func portRows(ports []GraphReportPortInput) []models.GraphReportPort {
	rows := make([]models.GraphReportPort, len(ports))
	for i, port := range ports {
		rows[i] = models.GraphReportPort{
			DeviceID: port.DeviceID, DeviceName: strings.TrimSpace(port.DeviceName),
			IfName: strings.TrimSpace(port.IfName), Label: strings.TrimSpace(port.Label), Position: i,
		}
	}
	return rows
}
