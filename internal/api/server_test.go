package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.hollow.sh/toolbox/ginjwt"

	"github.com/stretchr/testify/assert"
)

func TestAPILivenessCheck(t *testing.T) {
	t.Log("starting test")

	apiserver := Server{
		Conf: &Conf{
			AuthConf: []ginjwt.AuthConfig{
				{
					Enabled:  false,
					Audience: "audience",
					Issuer:   "issuer",
					JWKSURI:  "jwksuri",
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
			AuthConf: []ginjwt.AuthConfig{
				{
					Enabled:  false,
					Audience: "audience",
					Issuer:   "issuer",
					JWKSURI:  "jwksuri",
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
