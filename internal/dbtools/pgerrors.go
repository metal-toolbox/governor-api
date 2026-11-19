package dbtools

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

// PostgreSQL error codes that callers act on. The server uses lib/pq and the
// tests use pgx, so both error types are recognised.
const (
	pgUniqueViolation  = "23505"
	pgLockNotAvailable = "55P03"
	pgQueryCanceled    = "57014"
)

// pgError returns the SQLSTATE code and the constraint name of a PostgreSQL
// error from either driver. ok is false when err is not a PostgreSQL error.
func pgError(err error) (code, constraint string, ok bool) {
	var pgxErr *pgconn.PgError
	if errors.As(err, &pgxErr) {
		return pgxErr.Code, pgxErr.ConstraintName, true
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code), pqErr.Constraint, true
	}

	return "", "", false
}

// UniqueViolation reports whether err is a PostgreSQL unique violation, and
// returns the name of the violated constraint or unique index.
func UniqueViolation(err error) (constraint string, ok bool) {
	code, constraint, ok := pgError(err)
	if !ok || code != pgUniqueViolation {
		return "", false
	}

	return constraint, true
}

// IsTimeout reports whether err is a PostgreSQL lock timeout or statement
// timeout, or an expired context deadline.
func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	code, _, ok := pgError(err)

	return ok && (code == pgLockNotAvailable || code == pgQueryCanceled)
}
