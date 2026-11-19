package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RouterPollKindsFor is the jobs a role is polled with, discovery first so a
// new router has interfaces before its first counter poll.
func RouterPollKindsFor(role models.RouterRole) []models.RouterPollKind {
	base := []models.RouterPollKind{models.RouterPollKindDiscovery, models.RouterPollKindInterfaces}
	switch role {
	case models.RouterRoleBRAS:
		return append(base, models.RouterPollKindBRAS)
	case models.RouterRoleOther:
		return append(base, models.RouterPollKindBGP)
	default:
		return append(base, models.RouterPollKindBGP, models.RouterPollKindRoutes)
	}
}

func validRouterRole(role models.RouterRole) bool {
	switch role {
	case models.RouterRoleBorder, models.RouterRoleBRAS, models.RouterRoleOther:
		return true
	}
	return false
}

// SetRouterRole writes the role and brings the router's job rows in line with
// it. Rows that stay keep their schedule; new ones are due at once. BGP
// sessions go when the role stops polling them, since nothing would refresh
// them and the screen would keep showing stale state.
func SetRouterRole(tx *gorm.DB, routerID uuid.UUID, role models.RouterRole, byAdmin bool) error {
	err := tx.Model(&models.MikrotikRouter{}).Where("id = ?", routerID).
		Updates(map[string]interface{}{"role": role, "role_set_by_admin": byAdmin}).Error
	if err != nil {
		return err
	}
	kinds := RouterPollKindsFor(role)
	if err := tx.Where("router_id = ? AND kind NOT IN ?", routerID, kinds).
		Delete(&models.RouterPollJob{}).Error; err != nil {
		return err
	}
	now := time.Now()
	for _, kind := range kinds {
		job := models.RouterPollJob{RouterID: routerID, Kind: kind, DueAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&job).Error; err != nil {
			return err
		}
	}
	if hasRouterPollKind(kinds, models.RouterPollKindBGP) {
		return nil
	}
	return tx.Where("router_id = ?", routerID).Delete(&models.RouterBGPSession{}).Error
}

func hasRouterPollKind(kinds []models.RouterPollKind, kind models.RouterPollKind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}
