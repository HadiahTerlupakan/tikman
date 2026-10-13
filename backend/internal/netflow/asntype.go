package netflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ASNTypesURL is PeeringDB's network list, cut to the fields flowd keeps.
// Anonymous clients may fetch it about once an hour; an API key lifts that,
// so a manual check from the same host cannot starve flowd.
const ASNTypesURL = "https://www.peeringdb.com/api/net?fields=asn,info_type"

// minASNTypesNetworks guards against an answer that is not the full list.
// PeeringDB holds some 35,000 networks; a page cut short by an error or rate
// limit must not replace the types in use with a handful.
const minASNTypesNetworks = 1000

type peeringDBNets struct {
	Data []struct {
		ASN      uint32 `json:"asn"`
		InfoType string `json:"info_type"`
	} `json:"data"`
}

// ParseASNTypes reads PeeringDB's answer into AS number -> network type
// (Content, Cable/DSL/ISP, NSP, Enterprise, ...). Networks that state no type
// are left out.
func ParseASNTypes(r io.Reader) (map[uint32]string, error) {
	var nets peeringDBNets
	if err := json.NewDecoder(r).Decode(&nets); err != nil {
		return nil, fmt.Errorf("peeringdb: %w", err)
	}
	if len(nets.Data) < minASNTypesNetworks {
		return nil, fmt.Errorf("peeringdb: only %d networks, expected the full list", len(nets.Data))
	}
	types := make(map[uint32]string, len(nets.Data))
	for _, n := range nets.Data {
		if n.ASN != 0 && n.InfoType != "" {
			types[n.ASN] = n.InfoType
		}
	}
	if len(types) == 0 {
		return nil, errors.New("peeringdb: no network states its type")
	}
	return types, nil
}

// FetchASNTypes downloads and parses PeeringDB's network list, with apiKey
// when it is set. It keeps no copy on disk: the types live in the database,
// which is what the API reads.
func FetchASNTypes(client *http.Client, url, apiKey string) (map[uint32]string, error) {
	var header http.Header
	if apiKey != "" {
		header = http.Header{"Authorization": {"Api-Key " + apiKey}}
	}
	var body bytes.Buffer
	if err := fetchTo(client, url, header, &body); err != nil {
		return nil, err
	}
	return ParseASNTypes(&body)
}
