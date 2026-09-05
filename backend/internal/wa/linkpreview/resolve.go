package linkpreview

import (
	"context"
	"net/url"
	"time"
)

const (
	// pageLimit and imageLimit bound what a hostile or merely careless host can
	// make this process read.
	pageLimit  = 512 << 10 // 512 KiB of HTML is far more than a <head> needs
	imageLimit = 4 << 20   // 4 MiB before scaling
	// budget covers the page and the image together. Past this the message
	// goes out plain rather than late.
	budget = 6 * time.Second
)

// Preview is what a link resolved to, ready to hang on a message.
type Preview struct {
	URL         string
	Title       string
	Description string
	// Thumbnail is a JPEG, or nil when the page named no usable image.
	Thumbnail []byte
}

// Resolve turns the first link in a message into a preview, or returns nil.
//
// It has no error return, and that is the point. A preview is decoration: a
// site that is slow, down, blocked by the address guard, not HTML, or without
// any metadata all mean the same thing to the caller — send the message
// without a card, exactly as it did before this existed.
func Resolve(ctx context.Context, body string) *Preview {
	raw := firstURL(body)
	if raw == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	page, err := get(ctx, raw, pageLimit)
	if err != nil {
		return nil
	}

	og := parseOpenGraph(page)
	if og.Title == "" {
		// A card with no title is a grey box. Better to show the plain link.
		return nil
	}

	preview := &Preview{URL: raw, Title: og.Title, Description: og.Description}
	if og.Image == "" {
		return preview
	}

	imgURL, err := absoluteImageURL(raw, og.Image)
	if err != nil {
		return preview
	}
	// The image goes through the same guard as the page: og:image is content
	// the remote site controls, so it is no more trusted than the URL itself.
	imgBytes, err := get(ctx, imgURL, imageLimit)
	if err != nil {
		return preview
	}
	if thumb, err := thumbnail(imgBytes); err == nil {
		preview.Thumbnail = thumb
	}
	return preview
}

// absoluteImageURL resolves og:image against the page that declared it. Most
// sites give a path rather than a full address.
func absoluteImageURL(pageURL, image string) (string, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(image)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}
