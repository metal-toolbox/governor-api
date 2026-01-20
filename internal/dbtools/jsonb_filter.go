package dbtools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

// MetadataKeyPattern is the regex pattern for valid metadata keys.
// Keys must start with a letter, can contain alphanumeric characters, underscores,
// hyphens, and forward slashes, and must end with an alphanumeric character.
var MetadataKeyPattern = regexp.MustCompile(`^[a-zA-Z]([a-zA-Z0-9_\-/]+[a-zA-Z0-9])?$`)

// ParseJSONBFilterQuery parses a metadata query string in the format "key.path=value"
// and returns a SQLBoiler query modifier for filtering on the JSONB column.
//
// The columnName parameter specifies which JSONB column to query (e.g., "metadata").
//
// Example inputs:
//   - "team=platform" -> queries metadata->>'team' = 'platform'
//   - "labels.env=prod" -> queries metadata#>>'{labels,env}' = 'prod'
func ParseJSONBFilterQuery(columnName, queryString string) (qm.QueryMod, error) {
	const kvPartsLen = 2

	searchKV := strings.SplitN(queryString, "=", kvPartsLen)
	if len(searchKV) < kvPartsLen {
		return nil, fmt.Errorf("%w: %s", ErrInvalidMetadataQueryFormat, queryString)
	}

	searchKey, searchValue := searchKV[0], searchKV[1]
	pathComponents := strings.Split(searchKey, ".")

	for _, pc := range pathComponents {
		if !MetadataKeyPattern.MatchString(pc) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMetadataKey, pc)
		}
	}

	sqlPath := fmt.Sprintf("{%s}", strings.Join(pathComponents, ","))

	return qm.Where(columnName+"#>>? = ?", sqlPath, searchValue), nil
}

// ParseJSONBFilterQueries parses multiple metadata query strings and returns
// a slice of query modifiers. This is a convenience function for processing
// multiple query parameter values.
//
// If any query string fails to parse, an error is returned immediately.
func ParseJSONBFilterQueries(columnName string, queryStrings []string) ([]qm.QueryMod, error) {
	mods := make([]qm.QueryMod, 0, len(queryStrings))

	for _, qs := range queryStrings {
		mod, err := ParseJSONBFilterQuery(columnName, qs)
		if err != nil {
			return nil, err
		}

		mods = append(mods, mod)
	}

	return mods, nil
}

// IsValidMetadataKey checks if a single key component matches the metadata key pattern
func IsValidMetadataKey(key string) bool {
	return MetadataKeyPattern.MatchString(key)
}

// IsValidMetadata recursively validates that all keys in the metadata map
// follow the MetadataKeyPattern
func IsValidMetadata(metadata map[string]interface{}) bool {
	for key, value := range metadata {
		if !MetadataKeyPattern.MatchString(key) {
			return false
		}

		// If the value is a map, recursively validate its keys
		if nestedMap, ok := value.(map[string]interface{}); ok {
			if !IsValidMetadata(nestedMap) {
				return false
			}
		}

		// If the value is a slice, check if any element is a map and validate it
		if slice, ok := value.([]interface{}); ok {
			for _, item := range slice {
				if nestedMap, ok := item.(map[string]interface{}); ok {
					if !IsValidMetadata(nestedMap) {
						return false
					}
				}
			}
		}
	}

	return true
}
