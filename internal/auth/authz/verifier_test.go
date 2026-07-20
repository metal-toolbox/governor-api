package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/hollow-toolbox/ginauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errJWT     = errors.New("jwt boom")
	errDecider = errors.New("decider down")
)

// stubVerifier is a TokenVerifier for tests.
type stubVerifier struct {
	cm  ginauth.ClaimMetadata
	err error
}

func (s stubVerifier) VerifyToken(*gin.Context) (ginauth.ClaimMetadata, error) {
	return s.cm, s.err
}

// stubDecider is a Decider for tests.
type stubDecider struct {
	allow  bool
	err    error
	called bool
	gotReq AuthzRequest
}

func (s *stubDecider) Enabled() bool { return true }

func (s *stubDecider) Eval(_ context.Context, in AuthzRequest) (bool, error) {
	s.called = true
	s.gotReq = in

	return s.allow, s.err
}

func newTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1alpha1/users", nil)

	return c
}

func TestMostSpecificScope(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   string
	}{
		{"create", []string{"write", "create", "create:governor:users", oidcScope}, "create:governor:users"},
		{"read", []string{"read", "read:governor:groups", oidcScope}, "read:governor:groups"},
		{"openid only", []string{oidcScope}, ""},
		{"no colon", []string{"write", "create"}, ""},
		{"empty", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mostSpecificScope(tt.scopes))
		})
	}
}

func TestVerifier_VerifyTokenWithScopes(t *testing.T) {
	claims := ginauth.ClaimMetadata{Subject: "wl-principal", User: "wl-user", Roles: []string{"role"}}
	scopes := []string{"write", "create", "create:governor:users", oidcScope}

	t.Run("jwt failure returns error and skips the decider", func(t *testing.T) {
		dec := &stubDecider{allow: true}
		v := NewVerifier(stubVerifier{err: errJWT}, dec)

		_, err := v.VerifyTokenWithScopes(newTestContext(), scopes)
		require.ErrorIs(t, err, errJWT)
		assert.False(t, dec.called, "decider should not be consulted when authn fails")
	})

	t.Run("allow sets context keys", func(t *testing.T) {
		dec := &stubDecider{allow: true}
		v := NewVerifier(stubVerifier{cm: claims}, dec)
		c := newTestContext()

		cm, err := v.VerifyTokenWithScopes(c, scopes)
		require.NoError(t, err)
		assert.Equal(t, claims, cm)

		assert.True(t, dec.called)
		assert.Equal(t, "wl-principal", dec.gotReq.Principal)
		assert.Equal(t, "create:governor:users", dec.gotReq.Scope)

		assert.Equal(t, "wl-principal", c.GetString(contextKeySubject))
		assert.Equal(t, "wl-user", c.GetString(contextKeyUser))
		assert.Equal(t, []string{"role"}, c.GetStringSlice(contextKeyRoles))
	})

	t.Run("deny returns authorization error", func(t *testing.T) {
		dec := &stubDecider{allow: false}
		v := NewVerifier(stubVerifier{cm: claims}, dec)
		c := newTestContext()

		_, err := v.VerifyTokenWithScopes(c, scopes)
		require.Error(t, err)

		var authErr *ginauth.AuthError
		assert.True(t, errors.As(err, &authErr), "expected ginauth.AuthError, got %T", err)

		// context must not be populated on denial
		assert.Empty(t, c.GetString(contextKeySubject))
	})

	t.Run("decider error is propagated", func(t *testing.T) {
		dec := &stubDecider{err: errDecider}
		v := NewVerifier(stubVerifier{cm: claims}, dec)

		_, err := v.VerifyTokenWithScopes(newTestContext(), scopes)
		require.ErrorIs(t, err, errDecider)
	})
}

func TestVerifier_SetMetadata(t *testing.T) {
	claims := ginauth.ClaimMetadata{Subject: "s", User: "u", Roles: []string{"r"}}
	v := NewVerifier(stubVerifier{}, &stubDecider{})
	c := newTestContext()

	v.SetMetadata(c, claims)

	assert.Equal(t, "s", c.GetString(contextKeySubject))
	assert.Equal(t, "u", c.GetString(contextKeyUser))
	assert.Equal(t, []string{"r"}, c.GetStringSlice(contextKeyRoles))
}
