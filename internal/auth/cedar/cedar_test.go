package cedar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/metal-toolbox/auditevent"
	"github.com/metal-toolbox/governor-api/internal/auth/authz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 250 * time.Millisecond

func TestNoopDecider(t *testing.T) {
	d := authz.NoopDecider()

	assert.False(t, d.Enabled())

	allow, err := d.Eval(context.Background(), authz.AuthzRequest{Principal: "p", Scope: "read:governor:users"})
	assert.NoError(t, err)
	assert.False(t, allow)
}

func TestSidecarDecider_Eval(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantAllow  bool
		wantErr    error
		wantErrAny bool
	}{
		{
			name: "allow",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"decision":"Allow","diagnostics":{"reason":["notifications-addon"],"errors":[]}}`)
			},
			wantAllow: true,
		},
		{
			name: "deny",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"decision":"Deny","diagnostics":{"reason":[],"errors":[]}}`)
			},
			wantAllow: false,
		},
		{
			name: "non-2xx fails closed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantAllow: false,
			wantErr:   ErrCedarUnexpectedStatus,
		},
		{
			name: "malformed body fails closed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `not json`)
			},
			wantAllow: false,
			wantErr:   ErrCedarResponse,
		},
		{
			name: "timeout fails closed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(2 * testTimeout)

				_, _ = io.WriteString(w, `{"decision":"Allow"}`)
			},
			wantAllow:  false,
			wantErrAny: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			d := NewDecider(srv.URL, testTimeout)

			allow, err := d.Eval(context.Background(), authz.AuthzRequest{
				Principal: `principal:G000:prod:system:serviceaccount:governor:gov-notifications-addon`,
				Scope:     "create:governor:users",
			})

			assert.Equal(t, tt.wantAllow, allow)

			switch {
			case tt.wantErr != nil:
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected error %v, got %v", tt.wantErr, err)
			case tt.wantErrAny:
				require.Error(t, err)
			default:
				assert.NoError(t, err)
			}
		})
	}
}

func TestSidecarDecider_Eval_connectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	d := NewDecider(url, testTimeout)

	allow, err := d.Eval(context.Background(), authz.AuthzRequest{Principal: "p", Scope: "read:governor:users"})
	assert.False(t, allow)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCedarRequest))
}

func TestSidecarDecider_Eval_sendsCedarRequest(t *testing.T) {
	var got cedarRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, isAuthorizedPath, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))

		_, _ = io.WriteString(w, `{"decision":"Allow"}`)
	}))
	defer srv.Close()

	d := NewDecider(srv.URL, testTimeout)

	_, err := d.Eval(context.Background(), authz.AuthzRequest{
		Principal: "wl-principal",
		Scope:     "create:governor:users",
	})
	require.NoError(t, err)

	assert.Equal(t, `Workload::"wl-principal"`, got.Principal)
	assert.Equal(t, `Action::"create:governor:users"`, got.Action)
	assert.Equal(t, resourcePlaceholder, got.Resource)
}

// fakeAuditEventWriter is an AuditEventWriter for tests.
type fakeAuditEventWriter struct {
	events []*auditevent.AuditEvent
}

func (f *fakeAuditEventWriter) Write(e *auditevent.AuditEvent) error {
	f.events = append(f.events, e)
	return nil
}

func TestSidecarDecider_Eval_writesAuditEvent(t *testing.T) {
	t.Run("allow", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"decision":"Allow"}`)
		}))
		defer srv.Close()

		aw := &fakeAuditEventWriter{}
		d := NewDecider(srv.URL, testTimeout, WithAuditWriter(aw))

		allow, err := d.Eval(context.Background(), authz.AuthzRequest{Principal: "wl-principal", Scope: "create:governor:users"})
		require.NoError(t, err)
		assert.True(t, allow)

		require.Len(t, aw.events, 1)
		e := aw.events[0]
		assert.Equal(t, auditEventType, e.Type)
		assert.Equal(t, defaultComponent, e.Component)
		assert.Equal(t, auditevent.OutcomeSucceeded, e.Outcome)
		assert.Equal(t, "wl-principal", e.Subjects["principal"])
		assert.Equal(t, "create:governor:users", e.Target["scope"])
		assert.Equal(t, "Allow", e.Target["detail"])
	})

	t.Run("deny", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"decision":"Deny"}`)
		}))
		defer srv.Close()

		aw := &fakeAuditEventWriter{}
		d := NewDecider(srv.URL, testTimeout, WithAuditWriter(aw))

		allow, err := d.Eval(context.Background(), authz.AuthzRequest{Principal: "wl-principal", Scope: "create:governor:users"})
		require.NoError(t, err)
		assert.False(t, allow)

		require.Len(t, aw.events, 1)
		assert.Equal(t, auditevent.OutcomeDenied, aw.events[0].Outcome)
	})

	t.Run("failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		aw := &fakeAuditEventWriter{}
		d := NewDecider(srv.URL, testTimeout, WithAuditWriter(aw), WithComponent("custom-component"))

		_, err := d.Eval(context.Background(), authz.AuthzRequest{Principal: "wl-principal", Scope: "create:governor:users"})
		require.Error(t, err)

		require.Len(t, aw.events, 1)
		e := aw.events[0]
		assert.Equal(t, auditevent.OutcomeFailed, e.Outcome)
		assert.Equal(t, "custom-component", e.Component)
		assert.Contains(t, e.Target["detail"], ErrCedarUnexpectedStatus.Error())
	})

	t.Run("no writer configured is a no-op", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"decision":"Allow"}`)
		}))
		defer srv.Close()

		d := NewDecider(srv.URL, testTimeout)

		_, err := d.Eval(context.Background(), authz.AuthzRequest{Principal: "p", Scope: "read:governor:users"})
		require.NoError(t, err)
	})
}
