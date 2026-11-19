// Package openapi holds the governor-api OpenAPI spec. redocly lint validates
// the spec; the tests here check what the validator cannot know is required.
package openapi

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserSchemasDeprecateExternalID checks that every user schema that still
// carries the external_id field marks it deprecated and offers external_ids.
func TestUserSchemasDeprecateExternalID(t *testing.T) {
	v := viper.New()
	v.SetConfigFile("v1alpha1/components/schemas/users.yaml")
	require.NoError(t, v.ReadInConfig())

	tests := []struct {
		name   string
		schema string
	}{
		{name: "user", schema: "User"},
		{name: "user profile", schema: "UserProfile"},
		{name: "create request", schema: "UserCreateRequest"},
		{name: "update request", schema: "UserUpdateRequest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := tt.schema + ".properties."

			assert.Equal(t, true, v.Get(props+"external_id.deprecated"), "external_id is deprecated")
			assert.Equal(t, "array", v.Get(props+"external_ids.type"), "external_ids replaces it")
		})
	}
}
