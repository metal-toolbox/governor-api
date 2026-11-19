package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalIDJSON(t *testing.T) {
	b, err := json.Marshal(ExternalID{Iss: "https://idp.example.com", ID: "sub"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"iss":"https://idp.example.com","id":"sub"}`, string(b))
}
