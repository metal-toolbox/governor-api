package configs

import (
	"fmt"
	"strings"
)

// IdentityConfig controls how a sign-in from this provider maps to a governor user.
type IdentityConfig struct {
	// Enabled marks a provider whose tokens identify governor users. At least one
	// provider must set it. The first enabled provider in the oidc list is the
	// primary identity provider. Default false.
	Enabled bool `mapstructure:"enabled"`
	// AutoLink links an unknown (issuer, external ID) to the existing active user
	// whose email equals the OIDC userinfo email. Used only when Enabled. Default false.
	AutoLink bool `mapstructure:"autoLink"`
	// AutoRegister creates a user for an unknown sign-in that matches no user.
	// Used only when Enabled. nil means true: an unknown sign-in creates a user.
	AutoRegister *bool `mapstructure:"autoRegister"`
}

// AutoRegisterOrDefault returns AutoRegister, or true when it is unset.
func (i IdentityConfig) AutoRegisterOrDefault() bool {
	if i.AutoRegister == nil {
		return true
	}

	return *i.AutoRegister
}

// IdentityIssuers returns the issuers of every provider with Identity.Enabled,
// in config order. Providers that share an issuer and differ only by audience
// are one identity provider, so each issuer appears once.
func IdentityIssuers(auths []Auth) []string {
	issuers := []string{}
	seen := make(map[string]struct{}, len(auths))

	for _, a := range auths {
		if !a.Identity.Enabled {
			continue
		}

		if _, ok := seen[a.Issuer]; ok {
			continue
		}

		seen[a.Issuer] = struct{}{}
		issuers = append(issuers, a.Issuer)
	}

	return issuers
}

// ValidateIdentityProviders returns an error matching
// ErrConflictingIdentityConfig when providers that share an issuer have
// different identity settings, and ErrNoIdentityProvider when no provider has
// Identity.Enabled. The second error lists the configured issuers, so the
// operator can pick the one to flag.
func ValidateIdentityProviders(auths []Auth) error {
	if err := validateIssuerIdentity(auths); err != nil {
		return err
	}

	if len(IdentityIssuers(auths)) > 0 {
		return nil
	}

	issuers := []string{}
	seen := make(map[string]struct{}, len(auths))

	for _, a := range auths {
		if _, ok := seen[a.Issuer]; ok {
			continue
		}

		seen[a.Issuer] = struct{}{}
		issuers = append(issuers, a.Issuer)
	}

	if len(issuers) == 0 {
		return fmt.Errorf("%w (no oidc provider configured)", ErrNoIdentityProvider)
	}

	return fmt.Errorf("%w (configured issuers: %s)", ErrNoIdentityProvider, strings.Join(issuers, ", "))
}

// PrimaryIdentityIssuer returns the issuer of the first provider with
// Identity.Enabled. ok is false when no provider has it.
func PrimaryIdentityIssuer(auths []Auth) (issuer string, ok bool) {
	issuers := IdentityIssuers(auths)
	if len(issuers) == 0 {
		return "", false
	}

	return issuers[0], true
}

// identitySettings is the effective identity behaviour of a provider, with
// AutoRegister resolved to its default.
type identitySettings struct {
	enabled, autoLink, autoRegister bool
}

func settingsOf(i IdentityConfig) identitySettings {
	return identitySettings{enabled: i.Enabled, autoLink: i.AutoLink, autoRegister: i.AutoRegisterOrDefault()}
}

// validateIssuerIdentity returns ErrConflictingIdentityConfig, with the
// issuer, when two providers with the same issuer have different identity
// settings.
func validateIssuerIdentity(auths []Auth) error {
	first := make(map[string]identitySettings, len(auths))

	for _, a := range auths {
		got := settingsOf(a.Identity)

		want, seen := first[a.Issuer]
		if !seen {
			first[a.Issuer] = got
			continue
		}

		if got != want {
			return fmt.Errorf("%w (issuer: %s)", ErrConflictingIdentityConfig, a.Issuer)
		}
	}

	return nil
}
