package connectivity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Verbatim from the C300's running config: a type is declared over several
// lines that repeat its name, and only some of them carry a description.
const onuTypeConfig = `  onu-type ALL gpon
  onu-type H640GW5 gpon description 4eth,2port,4wifi
  onu-type H640GW5 gpon max-tcont 40
  onu-type H640GW5 gpon max-switch-perslot 32
  onu-type ZTE-F609 gpon description 4ETH,2POTS,WIFI
  onu-type ZTE-F609 gpon max-tcont 7
  onu-type ZTE-F609 gpon max-gemport 32
`

func TestParseZTEONUTypeDetails(t *testing.T) {
	types := ParseZTEONUTypeDetails(onuTypeConfig)

	assert.Equal(t, []ZTEONUType{
		{Name: "ALL", PON: "gpon"},
		{Name: "H640GW5", PON: "gpon", Description: "4eth,2port,4wifi", MaxTCONT: 40},
		{Name: "ZTE-F609", PON: "gpon", Description: "4ETH,2POTS,WIFI", MaxTCONT: 7, MaxGemport: 32},
	}, types)
}

// A type declared over many lines is one entry, not one per line.
func TestParseZTEONUTypeDetailsMergesRepeatedNames(t *testing.T) {
	assert.Len(t, ParseZTEONUTypeDetails(onuTypeConfig), 3)
}
