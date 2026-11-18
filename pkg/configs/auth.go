package configs

import (
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/spf13/viper"
)

// Auth is a single OIDC provider (already parsed and validated by ginjwt),
// optionally extended with a Cedar authorization block. When the Cedar block is
// enabled, callers from this issuer are authorized by the cedar-agent sidecar
// instead of by governor scopes; providers without a Cedar block are scope-gated.
// The Identity block says whether, and how, the provider's tokens identify
// governor users.
type Auth struct {
	ginjwt.AuthConfig
	Cedar    CedarConfig
	Identity IdentityConfig
}

// Validate validates the auth configuration.
func (a *Auth) Validate() error {
	return a.Cedar.Validate()
}

// GetAuthFromFlags builds the auth providers from flags/config. It reuses
// ginjwt.GetAuthConfigsFromFlags for OIDC parsing, enabled-filtering, and
// validation, then augments each returned provider with its (optional) Cedar
// and identity blocks matched by issuer and audience. It assumes
// RegisterViperOIDCFlags was called beforehand.
func GetAuthFromFlags(v *viper.Viper) ([]Auth, error) {
	authcfgs, err := ginjwt.GetAuthConfigsFromFlags(v)
	if err != nil {
		return nil, err
	}

	// oidcCedar captures just the fields needed to recover each provider's Cedar
	// and identity blocks from the raw "oidc" config, since ginjwt's parser
	// discards them.
	type oidcCedar struct {
		Issuer   string         `mapstructure:"issuer"`
		Audience string         `mapstructure:"audience"`
		Cedar    CedarConfig    `mapstructure:"cedar"`
		Identity IdentityConfig `mapstructure:"identity"`
	}

	// authKey identifies a provider. Providers may share an issuer and differ
	// only by audience, so key on both — keying by issuer alone would let one
	// provider's cedar or identity block bleed onto its same-issuer siblings.
	type authKey struct{ issuer, audience string }

	// ginjwt drops the cedar and identity sub-blocks, so re-read them and index
	// by (issuer, audience).
	var raw []oidcCedar
	if err := v.UnmarshalKey("oidc", &raw); err != nil {
		return nil, ErrInvalidAuthConfig
	}

	rawByKey := make(map[authKey]oidcCedar, len(raw))
	for _, r := range raw {
		rawByKey[authKey{r.Issuer, r.Audience}] = r
	}

	auths := make([]Auth, 0, len(authcfgs))

	for _, cfg := range authcfgs {
		r := rawByKey[authKey{cfg.Issuer, cfg.Audience}]

		a := Auth{AuthConfig: cfg, Cedar: r.Cedar, Identity: r.Identity}
		if err := a.Validate(); err != nil {
			return nil, err
		}

		auths = append(auths, a)
	}

	return auths, nil
}
