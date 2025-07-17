package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

func TestCreateGroupRequestValidator(t *testing.T) {
	tests := map[string]struct {
		input     models.Group
		expectErr error
	}{
		"Empty": {
			models.Group{
				Description: "",
				Name:        "",
			},
			ErrEmptyInput,
		},
		"InvalidChar": {
			models.Group{
				Description: "n/a",
				Name:        "john's-secret-group",
			},
			ErrInvalidChar,
		},
		"Valid": {
			models.Group{
				Description: "n/a",
				Name:        "ajZ9-( A )[0] &.",
			},
			nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, actualErr := createGroupRequestValidator(&test.input)
			assert.ErrorIs(t, actualErr, test.expectErr)
		})
	}
}
