package services

import (
	"path/filepath"
	"strings"
)

// MediaPathWithin joins a stored relative path onto the media root and
// reports whether the result names something inside it. The paths come from
// rows the wa process wrote from what WhatsApp sent, so serving a file and
// deleting one must pass the same check. The root itself is refused: no
// attachment is stored as the directory, and serving it would list it.
func MediaPathWithin(root, rel string) (string, bool) {
	cleanRoot := filepath.Clean(root)
	full := filepath.Join(cleanRoot, rel)
	if !strings.HasPrefix(full, cleanRoot+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}
