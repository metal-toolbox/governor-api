package jsonschema

import (
	"context"
	"fmt"
	"strings"

	"github.com/metal-toolbox/governor-api/internal/models"
	jsonschemav6 "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

// Compiler is a struct for a JSON schema compiler
type Compiler struct {
	jsonschemav6.Compiler

	extensionID   string
	erdSlugPlural string
	version       string

	schemaExts []SchemaExtension
}

// Option is a functional configuration option for JSON schema compiler
type Option func(c *Compiler)

// NewCompiler configures and creates a new JSON schema compiler
func NewCompiler(
	extensionID, slugPlural, version string,
	opts ...Option,
) *Compiler {
	c := &Compiler{*jsonschemav6.NewCompiler(), extensionID, slugPlural, version, []SchemaExtension{}}

	c.Compiler.AssertFormat()
	c.Compiler.AssertContent()
	c.Compiler.AssertVocabs()

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithUniqueConstraint enables the unique constraint extension for a JSON
// schema. An extra `unique` field can be added to the JSON schema, and the
// Validator will ensure that the combination of every properties in the
// array is unique within the given extension resource definition.
// Note that unique constraint validation will be skipped if db is nil.
func WithUniqueConstraint(
	ctx context.Context,
	extensionResourceDefinition *models.ExtensionResourceDefinition,
	resourceID *string,
	db boil.ContextExecutor,
) Option {
	return func(c *Compiler) {
		uc := &UniqueConstraintCompiler{extensionResourceDefinition, resourceID, ctx, db}
		c.schemaExts = append(c.schemaExts, uc)
	}
}

func (c *Compiler) schemaURL() string {
	return fmt.Sprintf(
		"https://governor/extensions/%s/erds/%s/%s/schema.json",
		c.extensionID, c.erdSlugPlural, c.version,
	)
}

// Compile compiles the schema string
func (c *Compiler) Compile(schemaStr string) (*jsonschemav6.Schema, error) {
	url := c.schemaURL()

	schemaJSON, err := jsonschemav6.UnmarshalJSON(strings.NewReader(schemaStr))
	if err != nil {
		return nil, err
	}

	if err := c.AddResource(url, schemaJSON); err != nil {
		return nil, err
	}

	for _, se := range c.schemaExts {
		v, err := se.Compile()
		if err != nil {
			return nil, err
		}

		c.RegisterVocabulary(v)
	}

	return c.Compiler.Compile(url)
}
