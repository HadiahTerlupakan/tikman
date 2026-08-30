package api

// SetSettingRequest carries a new value for a setting. The value is never
// echoed back.
type SetSettingRequest struct {
	Value string `json:"value" binding:"required"`
}
