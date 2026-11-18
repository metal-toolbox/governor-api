package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// BackfillVersion is the goose version of the users.external_id backfill.
// A database at BackfillVersion-1 has the user_external_ids table and still
// reads users.external_id, so data loaded at that version is backfilled.
const BackfillVersion = 7

// maxExternalIDLength matches the user_external_ids_external_id_len check in 00006.
const maxExternalIDLength = 256

// ErrIdentityProviderCount is returned by migration 00007, up or down, unless
// exactly one oidc provider has identity.enabled. goose then rolls the
// migration back and does not record it.
var ErrIdentityProviderCount = errors.New("migration 00007 needs exactly one oidc provider with identity.enabled")

// ErrExternalIDLength is returned by migration 00007 when an active user's
// users.external_id is empty or longer than user_external_ids accepts. The
// error names the users whose external_id must be fixed first.
var ErrExternalIDLength = errors.New("migration 00007 needs every active users.external_id to be 1 to 256 characters")

// newBackfillMigration returns Go migration 00007. Up copies each active user's
// users.external_id into user_external_ids under the one identity issuer. Down
// copies that issuer's records back into users.external_id and deletes them; it
// runs after the column drop has been rolled back, which re-adds the column empty.
func newBackfillMigration(identityIssuers []string, logger *zap.Logger) *goose.Migration {
	up := func(ctx context.Context, tx *sql.Tx) error {
		iss, err := soleIssuer(identityIssuers)
		if err != nil {
			return err
		}

		if err := checkExternalIDLengths(ctx, tx); err != nil {
			return err
		}

		res, err := tx.ExecContext(ctx,
			`INSERT INTO user_external_ids (user_id, iss, external_id, created_at, updated_at)
			SELECT id, $1, external_id, now(), now()
			FROM users
			WHERE external_id IS NOT NULL AND deleted_at IS NULL
			ON CONFLICT DO NOTHING`,
			iss,
		)
		if err != nil {
			return fmt.Errorf("backfill user_external_ids: %w", err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("count backfilled rows: %w", err)
		}

		logger.Info("backfilled user external ids", zap.Int64("backfilled_rows", n), zap.String("iss", iss))

		return nil
	}

	down := func(ctx context.Context, tx *sql.Tx) error {
		iss, err := soleIssuer(identityIssuers)
		if err != nil {
			return err
		}

		res, err := tx.ExecContext(ctx,
			`UPDATE users u SET external_id = uei.external_id
			FROM user_external_ids uei
			WHERE uei.user_id = u.id AND uei.iss = $1`,
			iss,
		)
		if err != nil {
			return fmt.Errorf("restore users.external_id: %w", err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("count restored rows: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM user_external_ids WHERE iss = $1`, iss); err != nil {
			return fmt.Errorf("delete backfilled user_external_ids: %w", err)
		}

		logger.Info("restored users.external_id", zap.Int64("restored_rows", n), zap.String("iss", iss))

		return nil
	}

	m := goose.NewGoMigration(BackfillVersion, &goose.GoFunc{RunTx: up}, &goose.GoFunc{RunTx: down})
	// goose prints Source in migration results; without it the line names no file.
	m.Source = "00007_backfill_user_external_ids.go"

	return m
}

// soleIssuer returns the only identity issuer. The check does not depend on
// the row count, so a misconfigured provider list fails on every database.
func soleIssuer(identityIssuers []string) (string, error) {
	switch len(identityIssuers) {
	case 1:
		return identityIssuers[0], nil
	case 0:
		return "", fmt.Errorf("%w, found 0", ErrIdentityProviderCount)
	default:
		return "", fmt.Errorf("%w, found %d: %s",
			ErrIdentityProviderCount, len(identityIssuers), strings.Join(identityIssuers, ", "))
	}
}

// checkExternalIDLengths returns ErrExternalIDLength, with the IDs of the
// users concerned, when an active user's external_id would break the length
// check of user_external_ids. Without it the INSERT fails on a bare
// constraint violation that names no user.
func checkExternalIDLengths(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM users
		WHERE external_id IS NOT NULL AND deleted_at IS NULL
		AND char_length(external_id) NOT BETWEEN 1 AND $1
		ORDER BY id`,
		maxExternalIDLength,
	)
	if err != nil {
		return fmt.Errorf("check users.external_id lengths: %w", err)
	}

	defer rows.Close()

	var ids []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("check users.external_id lengths: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("check users.external_id lengths: %w", err)
	}

	if len(ids) > 0 {
		return fmt.Errorf("%w; fix the external_id of users %s", ErrExternalIDLength, strings.Join(ids, ", "))
	}

	return nil
}
