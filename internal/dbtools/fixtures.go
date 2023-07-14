//go:build testtools
// +build testtools

package dbtools

import (
	"github.com/metal-toolbox/governor-api/internal/models"
)

//nolint:revive
var (
	FixtureUsers models.UserSlice
)

func addFixtures() error {
	return nil
}
