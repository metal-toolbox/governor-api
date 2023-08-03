package dbtools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"golang.org/x/sync/errgroup"
)

// ErrDBUpdateNotificationPreferences is returned when there's an error occurred
// while updating the user notification preferences on database
var ErrDBUpdateNotificationPreferences = errors.New("an error occurred while updating the user notification preferences")

func newErrDBUpdateNotificationPreferences(msg string) error {
	return fmt.Errorf("%w: %s", ErrDBUpdateNotificationPreferences, msg)
}

// UserNotificationPreferenceTarget is the user notification target response
type UserNotificationPreferenceTarget struct {
	Target  string `json:"target"`
	Enabled *bool  `json:"enabled"`
}

// UserNotificationPreferenceTargets is an alias for user notification target
// slice
type UserNotificationPreferenceTargets []*UserNotificationPreferenceTarget

// UserNotificationPreference is the user notification preference response
type UserNotificationPreference struct {
	NotificationType string `json:"notification_type" boil:"notification_type"`
	Enabled          *bool  `json:"enabled"`

	NotificationTargets UserNotificationPreferenceTargets `json:"notification_targets"`
}

// UserNotificationPreferences is an alias for user notification
// preference slice
type UserNotificationPreferences []*UserNotificationPreference

// RefreshNotificationDefaults refreshes the notification_defaults
// materialized view
func RefreshNotificationDefaults(ctx context.Context, ex boil.ContextExecutor) (err error) {
	q := queries.Raw("REFRESH MATERIALIZED VIEW notification_defaults")
	_, err = q.ExecContext(ctx, ex)

	return
}

// GetNotificationPreferences fetch a user's notification preferences from
// the governor DB, this function only gets rules the user had modified if
// `withDefaults` is false
func GetNotificationPreferences(ctx context.Context, uid string, ex boil.ContextExecutor, withDefaults bool) (UserNotificationPreferences, error) {
	type preferencesQueryRecord struct {
		NotificationType        string          `boil:"notification_type"`
		NotificationTargetsJSON json.RawMessage `boil:"notification_targets"`
	}

	type preferencesQueryRecordNotificationTarget struct {
		Target  string `json:"f1"`
		Enabled *bool  `json:"f2"`
	}

	np := `
		WITH np as (
			SELECT 
				user_id,
				notification_types.id as type_id,
				notification_types.slug as type_slug,
				notification_preferences.notification_target_id_null_string as target_id,
				notification_targets.slug as target_slug,
				enabled
			FROM notification_preferences
			LEFT JOIN notification_targets ON notification_target_id = notification_targets.id
			LEFT JOIN notification_types ON notification_type_id = notification_types.id
			WHERE notification_preferences.user_id = $1
		)
	`

	qWithDefaults := fmt.Sprintf(
		"%s\n%s", np,
		`
		SELECT
			nd.type_slug AS notification_type,
			jsonb_agg((nd.target_slug, IFNULL(np.enabled, nd.default_enabled))) AS notification_targets
		FROM notification_defaults as nd
		FULL OUTER JOIN np on (np.target_id = nd.target_id AND np.type_id = nd.type_id)
		GROUP BY nd.type_slug
		`,
	)

	qWithoutDefaults := fmt.Sprintf(
		"%s\n%s", np,
		`
		SELECT
			np.type_slug AS notification_type,
			jsonb_agg((np.target_slug, np.enabled)) AS notification_targets
		FROM np
		GROUP BY np.type_slug
		`,
	)

	var q *queries.Query
	if withDefaults {
		q = queries.Raw(qWithDefaults, uid)
	} else {
		q = queries.Raw(qWithoutDefaults, uid)
	}

	records := []*preferencesQueryRecord{}

	if err := q.Bind(ctx, ex, &records); err != nil {
		return nil, err
	}

	eg := &errgroup.Group{}
	preferences := make(UserNotificationPreferences, len(records))

	for i, r := range records {
		i, r := i, r // https://golang.org/doc/faq#closures_and_goroutines

		eg.Go(func() error {
			targets := []*preferencesQueryRecordNotificationTarget{}
			if err := json.Unmarshal(r.NotificationTargetsJSON, &targets); err != nil {
				return err
			}

			p := new(UserNotificationPreference)
			p.NotificationType = r.NotificationType
			p.NotificationTargets = UserNotificationPreferenceTargets{}

			for _, t := range targets {
				// for rows with NULL target indicates configs are for the parent notification type
				if t.Target == "" {
					p.Enabled = t.Enabled
					continue
				}

				target := new(UserNotificationPreferenceTarget)
				*target = UserNotificationPreferenceTarget(*t)
				p.NotificationTargets = append(p.NotificationTargets, target)
			}

			preferences[i] = p
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, newErrDBUpdateNotificationPreferences(err.Error())
	}

	return preferences, nil
}

func slugToIDMap(ctx context.Context, table string, db boil.ContextExecutor) (slugIDMap map[string]string, err error) {
	rec := &struct {
		Map json.RawMessage `boil:"map"`
	}{}

	q := models.NewQuery(
		qm.Select(
			"jsonb_object_agg(slug, id) as map",
		),
		qm.From(table),
		qm.Where("deleted_at IS NULL"),
	)

	if err = q.Bind(ctx, db, rec); err != nil {
		return
	}

	err = json.Unmarshal(rec.Map, &slugIDMap)

	return
}

// CreateOrUpdateNotificationPreferences updates a user's notification
// preferences, or creates the preferences if not exists
func CreateOrUpdateNotificationPreferences(
	ctx context.Context,
	user *models.User,
	notificationPreferences UserNotificationPreferences,
	ex boil.ContextExecutor,
	auditID string,
	actor *models.User,
) (*models.AuditEvent, error) {
	typeSlugToID, err := slugToIDMap(ctx, models.TableNames.NotificationTypes, ex)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	targetSlugToID, err := slugToIDMap(ctx, models.TableNames.NotificationTargets, ex)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	curr, err := GetNotificationPreferences(ctx, user.ID, ex, false)
	if err != nil {
		return nil, err
	}

	for _, p := range notificationPreferences {
		notificationTypeID, ok := typeSlugToID[p.NotificationType]
		if !ok {
			return nil,
				newErrDBUpdateNotificationPreferences(fmt.Sprintf("notificationType %s not found", p.NotificationType))
		}

		if p.Enabled == nil {
			return nil, newErrDBUpdateNotificationPreferences(fmt.Sprintf(
				"notification type %s enabled value cannot be empty",
				p.NotificationType,
			))
		}

		np := &models.NotificationPreference{
			UserID:               user.ID,
			NotificationTypeID:   notificationTypeID,
			NotificationTargetID: null.NewString("", false),
			Enabled:              *p.Enabled,
		}

		if err := np.Upsert(
			ctx,
			ex,
			true,
			[]string{
				models.NotificationPreferenceColumns.UserID,
				models.NotificationPreferenceColumns.NotificationTypeID,
				models.NotificationPreferenceColumns.NotificationTargetIDNullString,
			},
			boil.Whitelist(models.NotificationPreferenceColumns.Enabled),
			boil.Infer(),
		); err != nil {
			return nil, err
		}

		for _, t := range p.NotificationTargets {
			notificationTargetID, ok := targetSlugToID[t.Target]
			if !ok {
				return nil,
					newErrDBUpdateNotificationPreferences(fmt.Sprintf("notificationTarget %s not found", t.Target))
			}

			if t.Enabled == nil {
				return nil, newErrDBUpdateNotificationPreferences(fmt.Sprintf(
					"notification type [%s] target [%s] enabled value cannot be empty",
					p.NotificationType,
					t.Target,
				))
			}

			np := &models.NotificationPreference{
				UserID:               user.ID,
				NotificationTypeID:   notificationTypeID,
				NotificationTargetID: null.NewString(notificationTargetID, true),
				Enabled:              *t.Enabled,
			}

			if err := np.Upsert(
				ctx,
				ex,
				true,
				[]string{
					models.NotificationPreferenceColumns.UserID,
					models.NotificationPreferenceColumns.NotificationTypeID,
					models.NotificationPreferenceColumns.NotificationTargetIDNullString,
				},
				boil.Whitelist(models.NotificationPreferenceColumns.Enabled),
				boil.Infer(),
			); err != nil {
				return nil, err
			}
		}
	}

	audit, err := AuditNotificationPreferencesUpdated(
		ctx, ex, auditID, actor, user.ID, curr, notificationPreferences,
	)
	if err != nil {
		return nil, err
	}

	return audit, nil
}
