package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMediaPathWithin(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "data", "wa-media")
	cases := []struct {
		name, rel, want string
		ok              bool
	}{
		{"a stored attachment", filepath.Join("2026", "11", "foto.jpg"), filepath.Join(root, "2026", "11", "foto.jpg"), true},
		{"a climb out of the root", filepath.Join("..", "etc", "passwd"), "", false},
		{"a sibling sharing the root's prefix", filepath.Join("..", "wa-media-lain", "x"), "", false},
		{"the root itself", ".", "", false},
		{"an empty path", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MediaPathWithin(root, tc.rel)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
