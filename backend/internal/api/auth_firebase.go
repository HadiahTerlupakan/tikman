package api

import (
	"net/http"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// tikmanClaim marks a token as minted by this backend for a signed-in TikMan
// user. database.rules.json requires it; the two must be changed together.
const tikmanClaim = "tikman"

// FirebaseTokenHandler mints the short-lived identity a browser needs before
// it may write its own presence node.
type FirebaseTokenHandler struct {
	app    *firebase.App
	logger *zap.Logger
}

// NewFirebaseTokenHandler constructs a FirebaseTokenHandler. A nil app means
// Firebase is not configured.
func NewFirebaseTokenHandler(app *firebase.App, logger *zap.Logger) *FirebaseTokenHandler {
	return &FirebaseTokenHandler{app: app, logger: logger}
}

// Token returns a Firebase custom token for the caller's own user id.
//
// The id is read from the session and never from the request: a token minted
// for an id the caller supplied would let any agent claim to be another, which
// is precisely what the RTDB rules use auth.uid to prevent.
func (h *FirebaseTokenHandler) Token(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	userID, ok := raw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}

	if h.app == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error: "Firebase is not configured",
			Code:  "FIREBASE_NOT_CONFIGURED",
		})
		return
	}

	client, err := h.app.Auth(c.Request.Context())
	if err != nil {
		h.logger.Error("firebase auth client", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to mint token", Code: "TOKEN_FAILED",
		})
		return
	}

	// The claim is what the RTDB rules test, instead of a bare `auth != null`.
	// Only a holder of the service account can set it, so a browser that signed
	// in through any provider the console happens to have enabled — Anonymous,
	// Google, whatever gets switched on later — carries no such claim and is
	// refused. Presence security stops depending on a console toggle.
	token, err := client.CustomTokenWithClaims(
		c.Request.Context(), userID.String(), map[string]interface{}{tikmanClaim: true})
	if err != nil {
		h.logger.Error("mint firebase custom token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to mint token", Code: "TOKEN_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": token}})
}
