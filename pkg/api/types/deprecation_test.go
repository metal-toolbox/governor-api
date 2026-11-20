package types

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Link target must be a file of this repository, so that the link
// resolves once the branch is on main.
func TestExternalIDDeprecationLinkTargetsRepoFile(t *testing.T) {
	m := regexp.MustCompile(`^<https://github\.com/metal-toolbox/governor-api/blob/main/([^>]+)>; rel="deprecation"$`).
		FindStringSubmatch(ExternalIDDeprecationLink)
	require.Len(t, m, 2, "unexpected Link value %q", ExternalIDDeprecationLink)

	_, err := os.Stat(filepath.Join("..", "..", "..", filepath.FromSlash(m[1])))
	assert.NoError(t, err)
}
