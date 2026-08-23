package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func TestOLTHandler_Delete(t *testing.T) {
	handler, siteService, db := SetupOLTHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		olt := CreateTestOLT(t, db, site.ID, "To Delete", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/olts/"+olt.ID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

		handler.Delete(c)

		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify deletion
		oltService := services.NewOLTService(db, testEncryptionKey)
		_, err = oltService.GetByID(olt.ID)
		assert.Error(t, err)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/olts/invalid", nil)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}

		handler.Delete(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
