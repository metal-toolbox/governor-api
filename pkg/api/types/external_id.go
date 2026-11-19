// Package types holds the types that the governor API versions share and that
// client modules use to build requests and read responses.
package types

// ExternalID is one identity of a user at an identity provider: the issuer
// URL of the provider and the provider's ID for the user.
type ExternalID struct {
	Iss string `json:"iss"`
	ID  string `json:"id"`
}
