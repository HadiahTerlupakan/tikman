package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPonHealthRejectsAnInvalidOLTID(t *testing.T) {
	handler, _, _ := SetupOLTHandlerTest(t)

	w, c := SetupTestContext("GET", "/api/v1/olts/bukan-uuid/pon-health", nil)
	c.Params = gin.Params{{Key: "id", Value: "bukan-uuid"}}
	handler.PonHealth(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPonHealthReportsAnOLTThatDoesNotExist(t *testing.T) {
	handler, _, _ := SetupOLTHandlerTest(t)

	missing := uuid.New().String()
	w, c := SetupTestContext("GET", "/api/v1/olts/"+missing+"/pon-health", nil)
	c.Params = gin.Params{{Key: "id", Value: missing}}
	handler.PonHealth(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
