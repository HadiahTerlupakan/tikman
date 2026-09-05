package api

import (
	"net/http"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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

	token, err := client.CustomToken(c.Request.Context(), userID.String())
	if err != nil {
		h.logger.Error("mint firebase custom token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to mint token", Code: "TOKEN_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": token}})
}
