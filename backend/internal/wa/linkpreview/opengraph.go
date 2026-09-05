package linkpreview

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// urlPattern finds the first web address in a message. Deliberately narrow:
// only http and https, which are the only schemes the fetcher will accept.
var urlPattern = regexp.MustCompile(`https?://[^\s<>"]+`)

// trailing punctuation a sentence leaves attached to an address.
const trailing = `.,;:!?)]}'"`

// firstURL returns the first link in a message, or "" when there is none.
func firstURL(body string) string {
	found := urlPattern.FindString(body)
	return strings.TrimRight(found, trailing)
}

// openGraph is what a page says about itself.
type openGraph struct {
	Title       string
	Description string
	Image       string
}

// parseOpenGraph reads a page's own description of itself.
//
// It never fails. A page that is not HTML, or is truncated by the size cap
// mid-tag, yields an empty result — the caller then sends the message without
// a card, which is the same thing it does today.
func parseOpenGraph(body []byte) openGraph {
	var og openGraph
	var titleTag string

	tokens := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			if og.Title == "" {
				og.Title = strings.TrimSpace(titleTag)
			}
			return og
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := tokens.Token()
			switch tok.Data {
			case "meta":
				readMeta(tok, &og)
			case "title":
				if tokens.Next() == html.TextToken {
					titleTag = string(tokens.Text())
				}
			case "body":
				// Everything worth reading is in the head; stopping here means
				// a long page costs a few tags rather than all of them.
				if og.Title == "" {
					og.Title = strings.TrimSpace(titleTag)
				}
				return og
			}
		}
	}
}

func readMeta(tok html.Token, og *openGraph) {
	var key, content string
	for _, a := range tok.Attr {
		switch a.Key {
		case "property", "name":
			key = a.Val
		case "content":
			content = a.Val
		}
	}
	switch key {
	case "og:title":
		og.Title = strings.TrimSpace(content)
	case "og:description", "description":
		if og.Description == "" {
			og.Description = strings.TrimSpace(content)
		}
	case "og:image", "og:image:url":
		if og.Image == "" {
			og.Image = strings.TrimSpace(content)
		}
	}
}
