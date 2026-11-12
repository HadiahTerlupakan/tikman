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
// attachment the wa process is meant to send. An image's stacked graphs are
// far below it; anything near it means a graph was not what LibreNMS should send.
const maxGraphReportImageBytes = 16 << 20

// graphRangeLayout is how each graph's window is written above it.
const graphRangeLayout = "02/01/2006 15:04"

// graphPortsLinePrefix opens the caption line naming an image's ports.
const graphPortsLinePrefix = "\n\nPort: "

// GraphReportCaption is the report's message with the customer filled in.
func GraphReportCaption(message, customer string) string {
	if strings.TrimSpace(message) == "" {
		message = DefaultGraphReportMessage
	}
	return strings.ReplaceAll(message, customerPlaceholder, customer)
}

// GraphImageCaption is the text one image of a report is sent with: the
// report's message, then the ports that image shows. Every image carries it,
// so an image read on its own in the chat still says what it is.
func GraphImageCaption(message, customer string, ports []string) string {
	return GraphReportCaption(message, customer) + graphPortsLinePrefix + strings.Join(ports, ", ")
}

// ReportImage is one image of a report and the labels of the ports it shows.
type ReportImage struct {
	PNG   []byte
	Ports []string
}

// graphRange writes the window a graph covers, in WIB.
func graphRange(now time.Time) string {
	return now.Add(-graphWindow).In(wib).Format(graphRangeLayout) + " – " +
		now.In(wib).Format(graphRangeLayout) + " WIB"
}

// graphPortsPerImage is how many ports one image of a report holds. A report
// with ten ports stacked into one image shrank every graph to an unreadable
// strip on a phone, so a longer report is split across several images.
const graphPortsPerImage = 2

// renderReport fetches every port's graph and stacks them, graphPortsPerImage
// to an image, in the ports' order. It is all or nothing: one graph LibreNMS
// cannot draw fails the whole report, so no image is answered for half of it.
func renderReport(ctx context.Context, client *librenms.Client, ports []models.GraphReportPort, now time.Time) ([]ReportImage, error) {
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
	images := make([]ReportImage, 0, (len(sections)+graphPortsPerImage-1)/graphPortsPerImage)
	for start := 0; start < len(sections); start += graphPortsPerImage {
		chunk := sections[start:min(start+graphPortsPerImage, len(sections))]
		image, err := composeImage(chunk)
		if err != nil {
			return nil, err
		}
		labels := make([]string, len(chunk))
		for i, section := range chunk {
			labels[i] = section.Title
		}
		images = append(images, ReportImage{PNG: image, Ports: labels})
	}
	return images, nil
}

func composeImage(sections []librenms.Section) ([]byte, error) {
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

// PreviewImage is one image of a previewed report with the caption it would
// be sent with.
type PreviewImage struct {
	PNG     []byte
	Caption string
}

// Preview renders the images a report with these ports would be sent as now,
// each with its caption. The group is not needed for it and is not checked.
func (s *GraphReportSender) Preview(ctx context.Context, in GraphReportInput) ([]PreviewImage, error) {
	if err := validateGraphReportFields(in); err != nil {
		return nil, err
	}
	client, err := s.LibreNMS()
	if err != nil {
		return nil, err
	}
	images, err := renderReport(ctx, client, portRows(in.Ports), s.now())
	if err != nil {
		return nil, err
	}
	customer := strings.TrimSpace(in.CustomerName)
	previews := make([]PreviewImage, len(images))
	for i, image := range images {
		previews[i] = PreviewImage{PNG: image.PNG, Caption: GraphImageCaption(in.Message, customer, image.Ports)}
	}
	return previews, nil
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
	images, err := renderReport(ctx, client, []models.GraphReportPort{port}, s.now())
	if err != nil {
		return nil, err
	}
	return images[0].PNG, nil
}

func portRows(ports []GraphReportPortInput) []models.GraphReportPort {
	rows := make([]models.GraphReportPort, len(ports))
	for i, port := range ports {
		rows[i] = models.GraphReportPort{
			DeviceID: port.DeviceID, DeviceName: strings.TrimSpace(port.DeviceName),
			IfName: strings.TrimSpace(port.IfName), Label: strings.TrimSpace(port.Label), Position: i,
			NotifyStatus: port.NotifyStatus,
		}
	}
	return rows
}
