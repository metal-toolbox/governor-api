//go:build testtools
// +build testtools

package dbtools

import (
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

//nolint:revive
var (
	FixtureUsers models.UserSlice
)

func addFixtures() error {
	return nil
}
