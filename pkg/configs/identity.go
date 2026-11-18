package configs

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
	// Used only when Enabled. nil means true, which keeps today's behaviour.
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

// ValidateIdentityProviders returns ErrNoIdentityProvider when no provider has
// Identity.Enabled.
func ValidateIdentityProviders(auths []Auth) error {
	if len(IdentityIssuers(auths)) == 0 {
		return ErrNoIdentityProvider
	}

	return nil
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
