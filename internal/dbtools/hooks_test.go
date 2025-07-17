package dbtools

import (
	"testing"

	"github.com/stretchr/testify/assert"

	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

func TestCalculateChangeset(t *testing.T) {
	tests := []struct {
		description string
		original    interface{}
		new         interface{}
		expected    []string
	}{
		{
			description: "compare empty Group models",
			original:    &models.Group{},
			new:         &models.Group{},
			expected:    []string{},
		},
		{
			description: "compare empty Group model to nonempty name",
			original:    &models.Group{},
			new:         &models.Group{Name: "group"},
			expected:    []string{"Name: \"\" => \"group\""},
		},
		{
			description: "compare empty User model to nonempty email",
			original:    &models.User{},
			new:         &models.User{Email: "dev@null.zombocom"},
			expected:    []string{"Email: \"\" => \"dev@null.zombocom\""},
		},
	}

	for _, tt := range tests {
		got := calculateChangeset(tt.original, tt.new)
		assert.Equal(t, tt.expected, got, "test: %s\texpected: %v\tgot: %v\n", tt.description, tt.expected, got)
	}
}
