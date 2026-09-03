package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// With one number the label is noise; with several it is the difference
// between a CS knowing which of the company's numbers a customer is talking
// to and guessing at it.
func TestListNamesTheNumberEachThreadCameInOn(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)

	first := csAccount(t, db)
	second := models.WAAccount{Label: "CS Kedua", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&second).Error)

	_, err := svc.FindOrCreate(IncomingPeer{
		WAAccountID: first.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)
	_, err = svc.FindOrCreate(IncomingPeer{
		WAAccountID: second.ID, JID: "628999@s.whatsapp.net", Phone: "628999888777", Name: "Siti",
	})
	require.NoError(t, err)

	rows, err := svc.List(ConversationFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byPhone := map[string]string{}
	for _, row := range rows {
		byPhone[row.CustomerPhone] = row.WAAccountLabel
	}
	assert.Equal(t, first.Label, byPhone["628111222333"])
	assert.Equal(t, "CS Kedua", byPhone["628999888777"])
}

// The same customer writing to two of the company's numbers is two threads,
// not one. Merging them would mean a reply leaving from whichever number the
// merged row happened to carry.
func TestOneCustomerOnTwoNumbersIsTwoThreads(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)

	first := csAccount(t, db)
	second := models.WAAccount{Label: "CS Kedua", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&second).Error)

	onFirst, err := svc.FindOrCreate(IncomingPeer{
		WAAccountID: first.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)
	onSecond, err := svc.FindOrCreate(IncomingPeer{
		WAAccountID: second.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	assert.NotEqual(t, onFirst.ID, onSecond.ID)
	assert.Equal(t, first.ID, onFirst.WAAccountID)
	assert.Equal(t, second.ID, onSecond.WAAccountID)
}
