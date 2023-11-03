package jsonschema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CompilerTestSuite struct {
	suite.Suite
}

func (s *CompilerTestSuite) TestCompile() {
	tests := []struct {
		name          string
		expectedErr   string
		erdSlugPlural string
		erdVersion    string
		schema        string
	}{
		{
			name: "ok",
			schema: `{
				"$id": "v1.canada-geese.test-ex-1",
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"title": "Canada Goose",
				"type": "object",
				"unique": [
					"firstName",
					"lastName"
				],
				"required": [
					"firstName",
					"lastName"
				],
				"properties": {
					"firstName": {
						"type": "string",
						"description": "The goose's first name.",
						"ui": {
							"hide": true
						}
					},
					"lastName": {
						"type": "string",
						"description": "The goose's last name."
					},
					"age": {
						"description": "Age in years which must be equal to or greater than zero.",
						"type": "integer",
						"minimum": 0
					}
				}
			}`,
			expectedErr:   "",
			erdSlugPlural: "canada-geese",
			erdVersion:    "v1alpha1",
		},
		{
			name: "test schema with required missing",
			schema: `
			{
				"$id": "v1.hello-world.extension-example.governor.equinixmetal.com",
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"unique": [
					"name"
				],
				"properties": {
					"name": {
						"default": "world",
						"description": "hello, name",
						"type": "string"
					}
				},
				"title": "Greeting",
				"type": "object"
			}
			`,
			expectedErr:   `cannot apply unique constraint when "required" is not provided`,
			erdSlugPlural: "greetings",
			erdVersion:    "v1alpha1",
		},
	}

	for _, tt := range tests {
		compiler := NewCompiler(
			"extension-validator", tt.erdSlugPlural, tt.erdVersion,
			WithUniqueConstraint(
				context.Background(),
				nil, nil, nil,
			),
		)

		_, err := compiler.Compile(tt.schema)

		if tt.expectedErr != "" {
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tt.expectedErr)
		} else {
			s.Require().NoError(err)
		}
	}
}

func TestCompilerSuite(t *testing.T) {
	suite.Run(t, new(CompilerTestSuite))
}
