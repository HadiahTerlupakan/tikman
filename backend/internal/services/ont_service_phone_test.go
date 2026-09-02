package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// setupONTPhoneTest gives every phone test a site and an OLT to hang ONTs
// off of: Create rejects an ONT whose OLT does not exist.
func setupONTPhoneTest(t *testing.T) (*ONTService, *models.OLT) {
	t.Helper()

	db := setupTestDB(t)
	site, err := NewSiteService(db).Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)
	olt := newTestOLT(t, db, site.ID, "Test OLT", "192.168.1.1")

	return NewONTService(db), olt
}

func TestONTPhoneIsStoredInOneForm(t *testing.T) {
	svc, olt := setupONTPhoneTest(t)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN-PHONE-1",
		Phone:        "0812-3456-7890",
	}
	err := svc.Create(ont)
	require.NoError(t, err)
	assert.Equal(t, "6281234567890", ont.Phone, "however the technician typed it")
}

// Two ONTs claiming one number would send the CS to the wrong house. The
// partial unique index enforces it on Postgres; this check is what makes the
// rule hold in the SQLite tests and gives the operator a sentence rather than a
// constraint name.
func TestONTPhoneCannotBeClaimedTwice(t *testing.T) {
	svc, olt := setupONTPhoneTest(t)

	first := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN-PHONE-2",
		Phone:        "081234567890",
	}
	require.NoError(t, svc.Create(first))

	// Same number, typed in a different shape, on a different box.
	second := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        2,
		SerialNumber: "SN-PHONE-3",
		Phone:        "+6281234567890",
	}
	err := svc.Create(second)
	assert.ErrorIs(t, err, ErrValidation)
}

// Most ONTs have no number recorded, and empty is not a value that can collide.
func TestManyONTsMayHaveNoPhoneAtAll(t *testing.T) {
	svc, olt := setupONTPhoneTest(t)

	serials := []string{"SN-NOPHONE-1", "SN-NOPHONE-2"}
	for i, serial := range serials {
		ont := &models.ONT{
			OLTID:        olt.ID,
			PortID:       1,
			ONTID:        i + 1,
			SerialNumber: serial,
		}
		require.NoError(t, svc.Create(ont))
	}
}

// The chat-matching lookup reads onts.phone directly, so Update has to
// normalize just as Create does — a number saved through an edit in another
// shape would silently stop matching its subscriber's chats.
func TestONTPhoneUpdateIsNormalized(t *testing.T) {
	svc, olt := setupONTPhoneTest(t)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN-PHONE-UPDATE-1",
	}
	require.NoError(t, svc.Create(ont))

	updated, err := svc.Update(ont.ID, map[string]interface{}{"phone": "0812 3456 7890"})
	require.NoError(t, err)
	assert.Equal(t, "6281234567890", updated.Phone)
}

// The duplicate check on Update must catch a number already claimed by a
// different ONT, not just an update that happens to touch its own row.
func TestONTPhoneUpdateCannotStealAnotherONTsNumber(t *testing.T) {
	svc, olt := setupONTPhoneTest(t)

	first := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN-PHONE-UPDATE-2",
		Phone:        "081234567890",
	}
	require.NoError(t, svc.Create(first))

	second := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        2,
		SerialNumber: "SN-PHONE-UPDATE-3",
	}
	require.NoError(t, svc.Create(second))

	_, err := svc.Update(second.ID, map[string]interface{}{"phone": "+6281234567890"})
	assert.ErrorIs(t, err, ErrValidation)
}

// Re-saving an ONT's own number (unchanged, or retyped in another shape) must
// not trip the duplicate check against itself.
func TestONTPhoneUpdateAllowsKeepingItsOwnNumber(t *testing.T) {
	svc, olt := setupONTPhoneTest(t)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN-PHONE-UPDATE-4",
		Phone:        "081234567890",
	}
	require.NoError(t, svc.Create(ont))

	updated, err := svc.Update(ont.ID, map[string]interface{}{"phone": "+6281234567890"})
	require.NoError(t, err)
	assert.Equal(t, "6281234567890", updated.Phone)
}

// An operator removing the number stores "", not an error.
func TestONTPhoneUpdateCanBeCleared(t *testing.T) {
	svc, olt := setupONTPhoneTest(t)

	ont := &models.ONT{
		OLTID:        olt.ID,
		PortID:       1,
		ONTID:        1,
		SerialNumber: "SN-PHONE-UPDATE-5",
		Phone:        "081234567890",
	}
	require.NoError(t, svc.Create(ont))

	updated, err := svc.Update(ont.ID, map[string]interface{}{"phone": ""})
	require.NoError(t, err)
	assert.Equal(t, "", updated.Phone)
}
