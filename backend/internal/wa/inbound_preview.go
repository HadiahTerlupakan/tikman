package wa

import (
	"go.mau.fi/whatsmeow/proto/waE2E"

	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
)

// inboundPreview lifts the link card out of a message a customer sent.
//
// Their own WhatsApp fetched the page and attached the result, so this costs
// no request and shows the inbox exactly what the customer is looking at. The
// same shape the send path produces is reused, so a card reaching the browser
// looks the same whichever direction it came from.
func inboundPreview(msg *waE2E.Message) *linkpreview.Preview {
	ext := msg.GetExtendedTextMessage()
	if ext == nil {
		return nil
	}
	// The extended form also carries quotes, which have no card. A title is
	// what separates the two: without one there is nothing to draw but a box.
	if ext.GetMatchedText() == "" || ext.GetTitle() == "" {
		return nil
	}
	return &linkpreview.Preview{
		URL:         ext.GetMatchedText(),
		Title:       ext.GetTitle(),
		Description: ext.GetDescription(),
		Thumbnail:   ext.GetJPEGThumbnail(),
	}
}
