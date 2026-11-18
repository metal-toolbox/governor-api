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

// backfillVersion is the goose version of the users.external_id backfill.
const backfillVersion = 7

// ErrIdentityProviderCount is returned by migration 00007, up or down, unless
// exactly one oidc provider has identity.enabled. goose then rolls the
// migration back and does not record it.
var ErrIdentityProviderCount = errors.New("migration 00007 needs exactly one oidc provider with identity.enabled")

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

	m := goose.NewGoMigration(backfillVersion, &goose.GoFunc{RunTx: up}, &goose.GoFunc{RunTx: down})
	// goose prints Source in migration results; without it the line names no file.
	m.Source = "00007_backfill_user_external_ids.go"

	return m
}

// soleIssuer returns the only identity issuer. The check does not depend on
// the row count, so a misconfigured provider list fails on every database.
func soleIssuer(identityIssuers []string) (string, error) {
	if len(identityIssuers) != 1 {
		return "", fmt.Errorf("%w, found %d: %s",
			ErrIdentityProviderCount, len(identityIssuers), strings.Join(identityIssuers, ", "))
	}

	return identityIssuers[0], nil
}
