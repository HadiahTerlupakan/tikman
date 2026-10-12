package mikrotik

// The REST API sends every value as a string, flags included ("true"), and
// keeps RouterOS's dotted property names ("remote.address"). These structs
// mirror that; parse.go turns the strings into values.

// SystemResource is /rest/system/resource.
type SystemResource struct {
	Uptime      string `json:"uptime"`
	Version     string `json:"version"`
	CPULoad     string `json:"cpu-load"`
	FreeMemory  string `json:"free-memory"`
	TotalMemory string `json:"total-memory"`
	BoardName   string `json:"board-name"`
}

// Clock is /rest/system/clock. Only the offset is read: it is what turns the
// zone-less timestamps elsewhere into instants.
type Clock struct {
	GMTOffset string `json:"gmt-offset"`
}

// Interface is one row of /rest/interface.
type Interface struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Comment  string `json:"comment"`
	Disabled string `json:"disabled"`
	Running  string `json:"running"`
	Dynamic  string `json:"dynamic"`
}

// VLAN is one row of /rest/interface/vlan.
type VLAN struct {
	Name      string `json:"name"`
	VLANID    string `json:"vlan-id"`
	Interface string `json:"interface"`
	Disabled  string `json:"disabled"`
}

// Address is one row of /rest/ip/address.
type Address struct {
	Address   string `json:"address"`
	Interface string `json:"interface"`
	Disabled  string `json:"disabled"`
	Invalid   string `json:"invalid"`
}

// BGPConnection is one configured peer, from /rest/routing/bgp/connection.
type BGPConnection struct {
	Name          string `json:"name"`
	RemoteAddress string `json:"remote.address"`
	RemoteAS      string `json:"remote.as"`
	Disabled      string `json:"disabled"`
}

// BGPSession is one running session, from /rest/routing/bgp/session.
type BGPSession struct {
	Name          string `json:"name"`
	RemoteAddress string `json:"remote.address"`
	RemoteAS      string `json:"remote.as"`
	LocalAddress  string `json:"local.address"`
	Established   string `json:"established"`
	Uptime        string `json:"uptime"`
	PrefixCount   string `json:"prefix-count"`
	LastStarted   string `json:"last-started"`
	LastStopped   string `json:"last-stopped"`
}
