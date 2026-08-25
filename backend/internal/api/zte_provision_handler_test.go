package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

type fakeZTEProvisioner struct {
	registerJob  *models.ProvisioningJob
	configureJob *models.ProvisioningJob
	err          error
	gotRequest   models.ZTEGPONRegisterRequest
	gotONTID     uuid.UUID
}

func (f *fakeZTEProvisioner) RegisterAndConfigure(_ context.Context, req models.ZTEGPONRegisterRequest, _ uuid.UUID) (*models.ProvisioningJob, error) {
	f.gotRequest = req
	return f.registerJob, f.err
}

func (f *fakeZTEProvisioner) ConfigureExisting(_ context.Context, ontID uuid.UUID, req models.ZTEGPONRegisterRequest, _ uuid.UUID) (*models.ProvisioningJob, error) {
	f.gotONTID = ontID
	f.gotRequest = req
	return f.configureJob, f.err
}

func validZTEAPIRequest() zteProvisionRequest {
	return zteProvisionRequest{
		ZTEGPONRegisterRequest: models.ZTEGPONRegisterRequest{
			OLTID:           uuid.New(),
			Card:            1,
			PON:             3,
			ONUIDMode:       models.ZTEONUIDCustom,
			ONUID:           7,
			SerialNumber:    "HWTCB403E8A0",
			ONUType:         "HG8245H5",
			ServiceEnabled:  true,
			VLANMode:        "tag",
			ServiceType:     "internet",
			VLANID:          100,
			DownloadProfile: "100M",
			UploadProfile:   "100M",
			WANMode:         "pppoe",
			VLANProfile:     "INTERNET",
			PPPoEUsername:   "example-user",
			PPPoEPassword:   "secret-password",
		},
		Confirm: true,
	}
}

func TestZTEProvisionHandler_InvalidOLTID(t *testing.T) {
	h := NewZTEProvisionHandler(&fakeZTEProvisioner{})
	w, c := SetupTestContext(http.MethodPost, "/api/v1/olts/not-a-uuid/gpon/register", validZTEAPIRequest())
	c.Params = gin.Params{{Key: "olt_id", Value: "not-a-uuid"}}

	h.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"INVALID_ID"`)
}

func TestZTEProvisionHandler_UnsupportedVendor(t *testing.T) {
	fake := &fakeZTEProvisioner{err: errors.New("ZTE GPON registration supports only C300 or C320 OLTs")}
	h := NewZTEProvisionHandler(fake)
	req := validZTEAPIRequest()
	w, c := SetupTestContext(http.MethodPost, "/api/v1/olts/"+req.OLTID.String()+"/gpon/register", req)
	c.Params = gin.Params{{Key: "olt_id", Value: req.OLTID.String()}}

	h.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"UNSUPPORTED_VENDOR"`)
}

func TestZTEProvisionHandler_RequiresConfirmation(t *testing.T) {
	h := NewZTEProvisionHandler(&fakeZTEProvisioner{})
	req := validZTEAPIRequest()
	req.Confirm = false
	w, c := SetupTestContext(http.MethodPost, "/api/v1/olts/"+req.OLTID.String()+"/gpon/register", req)
	c.Params = gin.Params{{Key: "olt_id", Value: req.OLTID.String()}}

	h.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"NOT_CONFIRMED"`)
}

func TestZTEProvisionHandler_ValidationErrorDoesNotExposePassword(t *testing.T) {
	fake := &fakeZTEProvisioner{err: errors.New("VLAN ID must be in range 1-4094")}
	h := NewZTEProvisionHandler(fake)
	req := validZTEAPIRequest()
	req.VLANID = 0
	w, c := SetupTestContext(http.MethodPost, "/api/v1/olts/"+req.OLTID.String()+"/gpon/register", req)
	c.Params = gin.Params{{Key: "olt_id", Value: req.OLTID.String()}}

	h.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"VALIDATION_ERROR"`)
	assert.NotContains(t, w.Body.String(), req.PPPoEPassword)
}

func TestZTEProvisionHandler_ReturnsAcceptedJobAndRedactedPreview(t *testing.T) {
	jobID := uuid.New()
	fake := &fakeZTEProvisioner{registerJob: &models.ProvisioningJob{ID: jobID, ONTID: uuid.New(), Status: models.ProvisioningStatusSuccess}}
	h := NewZTEProvisionHandler(fake)
	req := validZTEAPIRequest()
	w, c := SetupTestContext(http.MethodPost, "/api/v1/olts/"+req.OLTID.String()+"/gpon/register", req)
	c.Params = gin.Params{{Key: "olt_id", Value: req.OLTID.String()}}
	c.Set("user_id", uuid.New())

	h.Register(c)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.NotContains(t, w.Body.String(), req.PPPoEPassword)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, jobID.String(), body["job_id"])
	assert.Equal(t, float64(req.ONUID), body["onu_id"])
	assert.Equal(t, models.ProvisioningStatusSuccess, body["status"])
	commands, ok := body["commands"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, commands[8], "<redacted>")
}

func TestZTEProvisionHandler_ConfigureExistingParsesONTID(t *testing.T) {
	ontID := uuid.New()
	fake := &fakeZTEProvisioner{configureJob: &models.ProvisioningJob{ID: uuid.New(), ONTID: ontID, Status: models.ProvisioningStatusRunning}}
	h := NewZTEProvisionHandler(fake)
	req := validZTEAPIRequest()
	w, c := SetupTestContext(http.MethodPost, "/api/v1/onts/"+ontID.String()+"/gpon/configure", req)
	c.Params = gin.Params{{Key: "ont_id", Value: ontID.String()}}
	c.Set("user_id", uuid.New())

	h.ConfigureExisting(c)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, ontID, fake.gotONTID)
}

func TestZTEProvisionHandler_ReturnsResolvedAutoONUInResponseAndPreview(t *testing.T) {
	jobID := uuid.New()
	fake := &fakeZTEProvisioner{registerJob: &models.ProvisioningJob{ID: jobID, ONTID: uuid.New(), ONUID: 3, Status: models.ProvisioningStatusSuccess}}
	h := NewZTEProvisionHandler(fake)
	req := validZTEAPIRequest()
	req.ONUIDMode = models.ZTEONUIDAuto
	req.ONUID = 0
	w, c := SetupTestContext(http.MethodPost, "/api/v1/olts/"+req.OLTID.String()+"/gpon/register", req)
	c.Params = gin.Params{{Key: "olt_id", Value: req.OLTID.String()}}

	h.Register(c)

	require.Equal(t, http.StatusAccepted, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(3), body["onu_id"])
	commands := body["commands"].([]interface{})
	assert.Contains(t, commands[2], "onu 3 type")
}
