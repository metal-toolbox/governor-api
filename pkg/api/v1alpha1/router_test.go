package v1alpha1

import (
	"testing"

	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/stretchr/testify/assert"

	"github.com/metal-toolbox/governor-api/pkg/configs"
)

func TestRouterAuthConfigFor(t *testing.T) {
	okta := configs.Auth{
		AuthConfig: ginjwt.AuthConfig{Issuer: "https://okta", Audience: "web"},
		Identity:   configs.IdentityConfig{Enabled: true},
	}
	oktaCLI := configs.Auth{AuthConfig: ginjwt.AuthConfig{Issuer: "https://okta", Audience: "cli"}}

	tests := []struct {
		name   string
		iss    string
		want   configs.Auth
		wantOK bool
	}{
		{name: "first provider of the issuer", iss: "https://okta", want: okta, wantOK: true},
		{name: "unknown issuer", iss: "https://ping", want: configs.Auth{}, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Router{}
			r.SetAuthConf(configs.AuthConfigs{okta, oktaCLI})

			got, ok := r.authConfigFor(tt.iss)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, configs.AuthConfigs{okta, oktaCLI}, r.AuthConf)
		})
	}
}
