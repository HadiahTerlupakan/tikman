package services

import "github.com/tikman/olt-provisioning/internal/models"

// AllRouterPollKinds is every router tier, discovery first so a new router has
// interfaces before its first counter poll.
var AllRouterPollKinds = []models.RouterPollKind{
	models.RouterPollKindDiscovery,
	models.RouterPollKindInterfaces,
	models.RouterPollKindBGP,
}
