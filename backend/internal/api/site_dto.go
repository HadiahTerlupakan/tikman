package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

type CreateSiteRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Location    string `json:"location"`
	Description string `json:"description"`
}

type UpdateSiteRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=2,max=255"`
	Location    *string `json:"location"`
	Description *string `json:"description"`
}

type SiteResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Location    string    `json:"location"`
	Description string    `json:"description"`
	OLTCount    int       `json:"olt_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ToSiteResponse(oltCount int64, site *models.Site) SiteResponse {
	return SiteResponse{
		ID:          site.ID,
		Name:        site.Name,
		Location:    site.Location,
		Description: site.Description,
		OLTCount:    int(oltCount),
		CreatedAt:   site.CreatedAt,
		UpdatedAt:   site.UpdatedAt,
	}
}
