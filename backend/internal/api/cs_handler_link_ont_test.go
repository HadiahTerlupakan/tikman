package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// putLinkONT issues PUT .../conversations/:id/ont as the CS user and returns
// the raw response, so each test only asserts what it actually cares about.
func (e *csHandlerEnv) putLinkONT(convID, ontID string) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"ont_id":"` + ontID + `"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cs/conversations/"+convID+"/ont", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.asUser(e.cs, models.UserRoleCS).ServeHTTP(rec, req)
	return rec
}

// Section 9.1 of the spec: linking a conversation to an ONT by hand is where
// the subscriber's number gets captured, not a separate data-entry project.
// This is the end-to-end proof that LinkONT actually calls the recording
// rule, not just that the rule itself is correct in isolation.
func TestLinkONTRecordsTheSubscribersPhoneOnTheONT(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "0812-3456-7890")
	target := env.ont(t, "")

	rec := env.putLinkONT(conv.ID.String(), target.ID.String())
	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		PhoneRecorded bool `json:"phone_recorded"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.True(t, payload.PhoneRecorded)

	stored, err := env.onts.GetByID(target.ID)
	require.NoError(t, err)
	assert.Equal(t, "6281234567890", stored.Phone, "in the same normalized form Create uses")
}

// An ONT that already has a number keeps it. RecordPhoneIfUnclaimed is unit
// tested for this directly; this proves LinkONT does not bypass that guard
// by, say, writing the phone itself instead of delegating to the service.
func TestLinkONTLeavesAnExistingPhoneUntouched(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628222@s.whatsapp.net", "628222333444")
	target := env.ont(t, "628999888777")

	rec := env.putLinkONT(conv.ID.String(), target.ID.String())
	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		PhoneRecorded bool `json:"phone_recorded"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.False(t, payload.PhoneRecorded)

	stored, err := env.onts.GetByID(target.ID)
	require.NoError(t, err)
	assert.Equal(t, "628999888777", stored.Phone, "the number already there, not the conversation's")
}

// The CS's decision to link stands even when the number is already claimed
// elsewhere: the conversation gets its ont_id, the response says the number
// was not recorded, and the ONT that actually holds the number is untouched.
func TestLinkONTStillLinksWhenThePhoneIsClaimedByAnotherONT(t *testing.T) {
	env := setupCSHandler(t)

	holder := env.ont(t, "628333444555")
	conv := env.conversation(t, "628333@s.whatsapp.net", "0833-344-4555") // same number, typed differently
	target := env.ont(t, "")

	rec := env.putLinkONT(conv.ID.String(), target.ID.String())
	require.Equal(t, http.StatusOK, rec.Code, "a collision must not fail the link")

	var payload struct {
		Data struct {
			ONTID string `json:"ont_id"`
		} `json:"data"`
		PhoneRecorded bool `json:"phone_recorded"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, target.ID.String(), payload.Data.ONTID, "the CS's choice is honored")
	assert.False(t, payload.PhoneRecorded)

	stillEmpty, err := env.onts.GetByID(target.ID)
	require.NoError(t, err)
	assert.Empty(t, stillEmpty.Phone)

	untouched, err := env.onts.GetByID(holder.ID)
	require.NoError(t, err)
	assert.Equal(t, "628333444555", untouched.Phone, "the ONT that actually holds the number is unaffected")
}
