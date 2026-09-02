package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// ConnectAccountRequest is the phone number an admin typed in to pair a
// WhatsApp number, in whatever form they wrote it.
type ConnectAccountRequest struct {
	Phone string `json:"phone" binding:"required"`
}

// ListAccounts answers every WhatsApp number the team can answer from.
func (h *CSHandler) ListAccounts(c *gin.Context) {
	rows, err := h.accounts.List()
	if err != nil {
		mapCSError(c, err, "ACCOUNT_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// Connect starts pairing a number by phone. The account is marked "pairing"
// immediately, before the wa process even sees the request, so a browser
// polling the list shows the change without delay; the eight-character
// linking code the admin types into WhatsApp arrives moments later over the
// SSE stream the browser already listens to.
func (h *CSHandler) Connect(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_ACCOUNT_ID")
	if !ok {
		return
	}
	var req ConnectAccountRequest
	if !bindJSON(c, &req) {
		return
	}

	msg, err := connectControlMessage(id, req.Phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_PHONE"})
		return
	}

	before, err := h.accounts.Get(id)
	if err != nil {
		mapCSError(c, err, "CONNECT_FAILED")
		return
	}
	if err := h.accounts.MarkPairing(id); err != nil {
		mapCSError(c, err, "CONNECT_FAILED")
		return
	}

	if err := h.publishControl(c, msg); err != nil {
		h.rollbackStatus(id, before.Status)
		refuseControl(c, "CONNECT_FAILED")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": string(models.WAAccountPairing)}})
}

// rollbackStatus puts an account back where it was after a control message
// that never left. Nothing else clears "pairing" — the wa process is what
// does that, and it is precisely the thing that did not hear the request — so
// without this the badge sits amber with no way back but another Connect.
func (h *CSHandler) rollbackStatus(id uuid.UUID, status models.WAAccountStatus) {
	if err := h.accounts.SetStatus(id, status); err != nil {
		h.logger.Error("Could not put the WhatsApp account status back",
			zap.String("account_id", id.String()), zap.Error(err))
	}
}

// refuseControl answers the one failure this pair of endpoints cannot paper
// over: the wa process holds the WhatsApp session, and a request it never
// received has not been accepted by anyone.
func refuseControl(c *gin.Context, code string) {
	c.JSON(http.StatusBadGateway, ErrorResponse{
		Error: "proses WhatsApp tidak bisa dihubungi, coba lagi",
		Code:  code,
	})
}

// connectControlMessage builds the control message Connect publishes, after
// normalizing the phone number the way PairPhone on the wa side requires:
// it rejects a number written with a leading zero, so 628... form is built
// here rather than left to fail on the other side of Redis.
func connectControlMessage(id uuid.UUID, rawPhone string) (wa.ControlMessage, error) {
	phone, err := utils.NormalizePhone(rawPhone)
	if err != nil {
		return wa.ControlMessage{}, err
	}
	return wa.ControlMessage{Action: wa.ControlConnect, AccountID: id.String(), Phone: phone}, nil
}

// Disconnect asks the wa process to give up the pairing. The wa process
// records the account as disconnected once the session is actually logged
// out — this endpoint only asks, it never touches the WhatsApp connection.
func (h *CSHandler) Disconnect(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_ACCOUNT_ID")
	if !ok {
		return
	}
	if _, err := h.accounts.Get(id); err != nil {
		mapCSError(c, err, "DISCONNECT_FAILED")
		return
	}

	if err := h.publishControl(c, wa.ControlMessage{Action: wa.ControlDisconnect, AccountID: id.String()}); err != nil {
		refuseControl(c, "DISCONNECT_FAILED")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": string(models.WAAccountDisconnected)}})
}

// publishControl sends one action to the wa process. Unlike the announcements
// on cs:events, this channel is the only way the request reaches the process
// that holds the session: there is no sweep behind it, so a failure here is
// the caller's to hear about.
func (h *CSHandler) publishControl(c *gin.Context, msg wa.ControlMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("Could not encode a WhatsApp control message", zap.Error(err))
		return err
	}
	if err := h.redis.Publish(c.Request.Context(), wa.ControlChannel, payload).Err(); err != nil {
		h.logger.Error("Could not publish a WhatsApp control message", zap.Error(err))
		return err
	}
	return nil
}
