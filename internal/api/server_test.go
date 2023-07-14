package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPILivenessCheck(t *testing.T) {
	apiserver := Server{}
	api := apiserver.NewAPI()

	router := api.Handler

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.TODO(), "GET", "/healthz/liveness", nil)

	router.ServeHTTP(w, req)

	statusOK := w.Code == http.StatusOK

	body, err := io.ReadAll(w.Body)
	pageOK := err == nil

	assert.NotEmpty(t, body)

	assert.True(t, statusOK)
	assert.True(t, pageOK)
}

func TestAPIHealthzCheck(t *testing.T) {
	apiserver := Server{}
	api := apiserver.NewAPI()

	router := api.Handler

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.TODO(), "GET", "/healthz", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"status":"UP"}`, w.Body.String())
}
