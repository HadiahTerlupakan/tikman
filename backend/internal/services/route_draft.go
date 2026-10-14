package services

import (
	"fmt"
	"strings"
)

// draftLocalPref writes, for the operator to review and run by hand, the
// RouterOS v7 commands that move outbound traffic for prefix onto alt's peer:
// a rule in that peer's input chain, placed before the chain's own
// local-pref rule so its bogon filters still apply first, then a route
// refresh, a check and the undo. TikMan never sends these itself.
func draftLocalPref(prefix string, alt RoutePath, peer *routePeer, activePref int64) string {
	if peer == nil || peer.inputFilter == "" {
		return ""
	}
	chain := peer.inputFilter
	comment := fmt.Sprintf("tikman: %s via %s", prefix, alt.Peer)
	pref := activePref + draftLocalPrefStep
	lines := []string{
		fmt.Sprintf("# Keluar ke %s lewat %s (AS path %d, sekarang local-pref %d)", prefix, alt.Peer, alt.PathLen, alt.LocalPref),
		fmt.Sprintf(`/routing/filter/rule add chain=%s comment="%s" \`, chain, comment),
		fmt.Sprintf(`  place-before=[find where chain="%s" and rule~"set bgp-local-pref" and !(comment~"^tikman")] \`, chain),
		fmt.Sprintf(`  rule="if (dst == %s) { set bgp-local-pref %d; accept; }"`, prefix, pref),
		fmt.Sprintf(`/routing/bgp/session refresh [find where name~"^%s"]`, alt.Peer),
		"# Cek: rute aktif harus pindah ke peer di atas",
		fmt.Sprintf("/routing/route print where dst-address=%s", prefix),
		"# Batalkan",
		fmt.Sprintf(`/routing/filter/rule remove [find where comment="%s"]`, comment),
		fmt.Sprintf(`/routing/bgp/session refresh [find where name~"^%s"]`, alt.Peer),
	}
	return strings.Join(lines, "\n")
}
