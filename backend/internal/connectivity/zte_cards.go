package connectivity

import (
	"regexp"
	"sort"
	"strconv"
)

// zteAddCard matches the board inventory the OLT keeps in its running config:
//
//	add-card rackno 1 shelfno 1 slotno 3 GTGH
//
// This is the only place the cards themselves are listed. Deriving them from
// where ONUs happen to live hides a board that is fitted but not yet in use.
var zteAddCard = regexp.MustCompile(`(?m)^\s*add-card\s+rackno\s+(\d+)\s+shelfno\s+(\d+)\s+slotno\s+(\d+)\s+(\S+)`)

// ZTECard is one line card fitted to the OLT.
type ZTECard struct {
	Rack  int    `json:"rack"`
	Shelf int    `json:"shelf"`
	Slot  int    `json:"slot"`
	Type  string `json:"type"`
}

// ParseZTECards lists the fitted cards in slot order.
func ParseZTECards(config string) []ZTECard {
	cards := make([]ZTECard, 0)
	seen := make(map[int]bool)

	for _, match := range zteAddCard.FindAllStringSubmatch(unwrapZTEOutput(config), -1) {
		rack, _ := strconv.Atoi(match[1])
		shelf, _ := strconv.Atoi(match[2])
		slot, _ := strconv.Atoi(match[3])
		if seen[slot] {
			continue
		}
		seen[slot] = true
		cards = append(cards, ZTECard{Rack: rack, Shelf: shelf, Slot: slot, Type: match[4]})
	}

	sort.Slice(cards, func(i, j int) bool { return cards[i].Slot < cards[j].Slot })

	return cards
}
