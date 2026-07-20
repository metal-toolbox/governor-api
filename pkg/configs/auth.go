package configs

import (
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/spf13/viper"
)

// Auth is a single OIDC provider (already parsed and validated by ginjwt),
// optionally extended with a Cedar authorization block. When the Cedar block is
// enabled, callers from this issuer are authorized by the cedar-agent sidecar
// instead of by governor scopes; providers without a Cedar block are scope-gated.
type Auth struct {
	ginjwt.AuthConfig
	Cedar CedarConfig
}

// Validate validates the auth configuration.
func (a *Auth) Validate() error {
	return a.Cedar.Validate()
}

// GetAuthFromFlags builds the auth providers from flags/config. It reuses
// ginjwt.GetAuthConfigsFromFlags for OIDC parsing, enabled-filtering, and
// validation, then augments each returned provider with its (optional) Cedar
// block matched by issuer. It assumes RegisterViperOIDCFlags was called
// beforehand.
func GetAuthFromFlags(v *viper.Viper) ([]Auth, error) {
	authcfgs, err := ginjwt.GetAuthConfigsFromFlags(v)
	if err != nil {
		return nil, err
	}

	// oidcCedar captures just the fields needed to recover each provider's Cedar
	// block from the raw "oidc" config, since ginjwt's parser discards it.
	type oidcCedar struct {
		Issuer   string      `mapstructure:"issuer"`
		Audience string      `mapstructure:"audience"`
		Cedar    CedarConfig `mapstructure:"cedar"`
	}

	// authKey identifies a provider. Providers may share an issuer and differ
	// only by audience, so key on both — keying by issuer alone would let one
	// provider's cedar block bleed onto its same-issuer siblings.
	type authKey struct{ issuer, audience string }

	// ginjwt drops the cedar sub-block, so re-read it and index by (issuer, audience).
	var raw []oidcCedar
	if err := v.UnmarshalKey("oidc", &raw); err != nil {
		return nil, ErrInvalidAuthConfig
	}

	cedarByKey := make(map[authKey]CedarConfig, len(raw))
	for _, r := range raw {
		cedarByKey[authKey{r.Issuer, r.Audience}] = r.Cedar
	}

	auths := make([]Auth, 0, len(authcfgs))

	for _, cfg := range authcfgs {
		a := Auth{AuthConfig: cfg, Cedar: cedarByKey[authKey{cfg.Issuer, cfg.Audience}]}
		if err := a.Validate(); err != nil {
			return nil, err
		}

		auths = append(auths, a)
	}

	return auths, nil
}
