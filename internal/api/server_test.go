package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/metal-toolbox/hollow-toolbox/ginjwt"

	"github.com/stretchr/testify/assert"

	"github.com/metal-toolbox/governor-api/pkg/configs"
)

func TestAPILivenessCheck(t *testing.T) {
	t.Log("starting test")

	apiserver := Server{
		Conf: &Conf{
			AuthConf: []configs.Auth{
				{
					AuthConfig: ginjwt.AuthConfig{
						Enabled:  false,
						Audience: "audience",
						Issuer:   "issuer",
						JWKSURI:  "jwksuri",
					},
				},
			},
		},
	}

	api := apiserver.NewAPI()

	router := api.Handler

	w := httptest.NewRecorder()

	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/healthz/liveness", nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("serving http")

	router.ServeHTTP(w, req)

	statusOK := w.Code == http.StatusOK

	body, err := io.ReadAll(w.Body)
	pageOK := err == nil

	t.Log("body", string(body))
	assert.NotEmpty(t, body)

	assert.True(t, statusOK)
	assert.True(t, pageOK)
}

func TestAPIHealthzCheck(t *testing.T) {
	apiserver := Server{
		Conf: &Conf{
			AuthConf: []configs.Auth{
				{
					AuthConfig: ginjwt.AuthConfig{
						Enabled:  false,
						Audience: "audience",
						Issuer:   "issuer",
						JWKSURI:  "jwksuri",
					},
				},
			},
		},
	}

	api := apiserver.NewAPI()

	router := api.Handler

	w := httptest.NewRecorder()

	req, err := http.NewRequestWithContext(context.TODO(), "GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"status":"UP"}`, w.Body.String())
}

func TestRoutersPrimaryIdentityIssuer(t *testing.T) {
	provider := func(issuer string, identity bool) configs.Auth {
		return configs.Auth{
			AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: issuer},
			Identity:   configs.IdentityConfig{Enabled: identity},
		}
	}

	tests := []struct {
		name  string
		auths []configs.Auth
		want  string
	}{
		{
			name: "first flagged provider after an unflagged one",
			auths: []configs.Auth{
				provider("https://hydra.example.com", false),
				provider("https://okta.example.com", true),
				provider("https://ping.example.com", true),
			},
			want: "https://okta.example.com",
		},
		{
			name:  "no flagged provider",
			auths: []configs.Auth{provider("https://hydra.example.com", false)},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{Conf: &Conf{AuthConf: tt.auths}}

			alpha, beta := s.routers()

			assert.Equal(t, []string{tt.want, tt.want}, []string{alpha.PrimaryIdentityIssuer, beta.PrimaryIdentityIssuer})
		})
	}
}

func TestRoutersAuthConf(t *testing.T) {
	auths := configs.AuthConfigs{
		{AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: "https://okta.example.com"}, Identity: configs.IdentityConfig{Enabled: true}},
		{AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: "https://hydra.example.com"}},
	}

	s := &Server{Conf: &Conf{AuthConf: auths}}

	alpha, beta := s.routers()

	// The v1alpha1 providers can only be set through SetAuthConf, which also
	// builds the issuer lookup of user resolution.
	assert.Equal(t, auths, alpha.AuthConf())
	assert.Equal(t, auths, beta.AuthConf)
}
