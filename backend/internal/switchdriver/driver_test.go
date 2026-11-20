package switchdriver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tikman/olt-provisioning/internal/models"
)

func TestRegisterTwicePanics(t *testing.T) {
	noop := func(context.Context, *models.Switch, string, string) (Reader, error) { return nil, nil }
	assert.Panics(t, func() { Register(models.SwitchVendorHuaweiCE, noop) })
}
