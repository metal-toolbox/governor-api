package dbtools

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestUniqueViolation(t *testing.T) {
	type result struct {
		constraint string
		ok         bool
	}

	tests := []struct {
		name string
		err  error
		want result
	}{
		{
			name: "pgx unique violation",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key_2"},
			want: result{constraint: "users_email_key_2", ok: true},
		},
		{
			name: "lib/pq unique violation",
			err:  &pq.Error{Code: "23505", Constraint: "user_external_ids_iss_external_id_key"},
			want: result{constraint: "user_external_ids_iss_external_id_key", ok: true},
		},
		{
			name: "wrapped unique violation",
			err:  fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: "c"}),
			want: result{constraint: "c", ok: true},
		},
		{
			name: "other postgres error",
			err:  &pgconn.PgError{Code: "23503", ConstraintName: "fk"},
		},
		{
			name: "not a postgres error",
			err:  errors.New("other"), //nolint:err113 // any error that is not from PostgreSQL
		},
		{
			name: "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint, ok := UniqueViolation(tt.err)
			assert.Equal(t, tt.want, result{constraint: constraint, ok: ok})
		})
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "pgx lock timeout", err: &pgconn.PgError{Code: "55P03"}, want: true},
		{name: "pgx statement timeout", err: &pgconn.PgError{Code: "57014"}, want: true},
		{name: "lib/pq lock timeout", err: &pq.Error{Code: "55P03"}, want: true},
		{name: "lib/pq statement timeout", err: &pq.Error{Code: "57014"}, want: true},
		{name: "context deadline", err: fmt.Errorf("query: %w", context.DeadlineExceeded), want: true},
		{name: "unique violation", err: &pgconn.PgError{Code: "23505"}},
		{name: "other error", err: errors.New("other")}, //nolint:err113 // any error that is not from PostgreSQL
		{name: "nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTimeout(tt.err))
		})
	}
}
