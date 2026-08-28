package connectivity

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// An ONU type is declared over several lines that share its name:
//
//	onu-type HG8245H5 gpon description 4eth,2port,4wifi
//	onu-type HG8245H5 gpon max-tcont 40
//	onu-type HG8245H5 gpon max-gemport 32
var zteONUTypeLine = regexp.MustCompile(`(?m)^\s*onu-type\s+(\S+)\s+(\S+)(.*)$`)

// ZTEONUType is one ONU type the OLT will accept in a registration command,
// with what its declaration says about the hardware. Description is the OLT's
// own free text ("4eth,2port,4wifi"), kept verbatim rather than parsed into
// port counts, because nothing enforces its shape.
type ZTEONUType struct {
	Name        string `json:"name"`
	PON         string `json:"pon"`
	Description string `json:"description,omitempty"`
	MaxTCONT    int    `json:"max_tcont,omitempty"`
	MaxGemport  int    `json:"max_gemport,omitempty"`
}

// ParseZTEONUTypeDetails collects each ONU type's declaration from the running
// config, merging the lines that repeat the same name.
func ParseZTEONUTypeDetails(config string) []ZTEONUType {
	byName := make(map[string]*ZTEONUType)
	order := make([]string, 0)

	for _, match := range zteONUTypeLine.FindAllStringSubmatch(unwrapZTEOutput(config), -1) {
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		onuType, seen := byName[name]
		if !seen {
			onuType = &ZTEONUType{Name: name, PON: strings.TrimSpace(match[2])}
			byName[name] = onuType
			order = append(order, name)
		}

		attribute := strings.TrimSpace(match[3])
		switch {
		case strings.HasPrefix(attribute, "description "):
			onuType.Description = strings.TrimSpace(strings.TrimPrefix(attribute, "description "))
		case strings.HasPrefix(attribute, "max-tcont "):
			onuType.MaxTCONT, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(attribute, "max-tcont ")))
		case strings.HasPrefix(attribute, "max-gemport "):
			onuType.MaxGemport, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(attribute, "max-gemport ")))
		}
	}

	types := make([]ZTEONUType, 0, len(order))
	for _, name := range order {
		types = append(types, *byName[name])
	}
	sort.Slice(types, func(i, j int) bool { return types[i].Name < types[j].Name })

	return types
}
