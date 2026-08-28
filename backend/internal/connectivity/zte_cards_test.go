package connectivity

import "testing"

// Verbatim from the C300. Card 4 is fitted and carries no ONU yet, which is
// exactly the case deriving cards from ONU locations cannot see.
const cardInventory = `
add-card rackno 1 shelfno 1 slotno 3 GTGH
add-card rackno 1 shelfno 1 slotno 4 GTGH
interface gpon-olt_1/3/1
`

func TestParseZTECardsListsEveryFittedCard(t *testing.T) {
	cards := ParseZTECards(cardInventory)

	want := []ZTECard{
		{Rack: 1, Shelf: 1, Slot: 3, Type: "GTGH"},
		{Rack: 1, Shelf: 1, Slot: 4, Type: "GTGH"},
	}
	if len(cards) != len(want) {
		t.Fatalf("got %+v, want %+v", cards, want)
	}
	for i, card := range cards {
		if card != want[i] {
			t.Errorf("card %d = %+v, want %+v", i, card, want[i])
		}
	}
}

func TestParseZTECardsIsEmptyWithoutAnInventory(t *testing.T) {
	if cards := ParseZTECards("interface gpon-olt_1/3/1\n"); len(cards) != 0 {
		t.Fatalf("got %+v, want none", cards)
	}
}
