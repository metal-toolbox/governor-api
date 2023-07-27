package dbtools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var errUpdateNotificationPreferencesDB = errors.New("updateNotificationPreferencesDBError")

func newErrUpdateNotificationPreferences(msg string) error {
	return fmt.Errorf("%w: %s", errUpdateNotificationPreferencesDB, msg)
}

// UserNotificationPreferenceTarget is the user notification target response
type UserNotificationPreferenceTarget struct {
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
}

// UserNotificationPreferenceTargets is an alias for user notification target
// slice
type UserNotificationPreferenceTargets []*UserNotificationPreferenceTarget

// UserNotificationPreference is the user notification preference response
type UserNotificationPreference struct {
	NotificationType string `json:"notification_type" boil:"notification_type"`
	Enabled          bool   `json:"enabled"`

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
		Enabled bool   `json:"f2"`
	}

	queryMods := []qm.QueryMod{
		qm.With(
			`np as (
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
				WHERE notification_preferences.user_id = ?
			)`,
			uid,
		),
	}

	if withDefaults {
		queryMods = append(queryMods, qm.Select("nd.type_slug AS notification_type"))
		queryMods = append(queryMods, qm.Select("jsonb_agg((nd.target_slug, IFNULL(np.enabled, nd.default_enabled))) AS notification_targets"))
		queryMods = append(queryMods, qm.From("notification_defaults as nd"))
		queryMods = append(queryMods, qm.FullOuterJoin("np on (np.target_id = nd.target_id AND np.type_id = nd.type_id)"))
		queryMods = append(queryMods, qm.GroupBy("nd.type_slug"))
	} else {
		queryMods = append(queryMods, qm.Select("np.type_slug AS notification_type"))
		queryMods = append(queryMods, qm.Select("jsonb_agg((np.target_slug, np.enabled)) AS notification_targets"))
		queryMods = append(queryMods, qm.From("np"))
		queryMods = append(queryMods, qm.GroupBy("np.type_slug"))
	}

	records := []*preferencesQueryRecord{}
	q := models.NewQuery(queryMods...)

	if err := q.Bind(ctx, ex, &records); err != nil {
		return nil, err
	}

	errs := ""
	wg := &sync.WaitGroup{}
	preferences := make(UserNotificationPreferences, len(records))
	buildNotificationPreferenceRecord := func(i int, r *preferencesQueryRecord) {
		defer wg.Done()

		targets := []*preferencesQueryRecordNotificationTarget{}
		if err := json.Unmarshal(r.NotificationTargetsJSON, &targets); err != nil {
			errs += fmt.Sprintf("%s\n", err.Error())
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
	}

	for i, r := range records {
		wg.Add(1)

		go buildNotificationPreferenceRecord(i, r)
	}

	wg.Wait()

	if errs != "" {
		return nil, newErrUpdateNotificationPreferences(errs)
	}

	return preferences, nil
}

func slugToIDMap(ctx context.Context, table string, db *sqlx.DB) (slugIDMap map[string]string, err error) {
	rec := &struct {
		Map json.RawMessage `boil:"map"`
	}{}

	q := models.NewQuery(
		qm.Select(
			"1 as tmp",
			"jsonb_object_agg(slug, id) as map",
		),
		qm.From(table),
		qm.Where("deleted_at IS NULL"),
		qm.GroupBy("1"),
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
	db *sqlx.DB,
	auditID string,
	actor *models.User,
) (*models.AuditEvent, error) {
	typeSlugToID := map[string]string{}
	targetSlugToID := map[string]string{}
	errs := ""
	wg := &sync.WaitGroup{}

	wg.Add(1)

	getTypeSlugToIDMap := func() {
		defer wg.Done()

		var err error

		typeSlugToID, err = slugToIDMap(ctx, models.TableNames.NotificationTypes, db)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			errs += fmt.Sprintf("%s\n", err)
		}
	}

	go getTypeSlugToIDMap()

	wg.Add(1)

	getTargetSlugToIDMap := func() {
		defer wg.Done()

		var err error

		targetSlugToID, err = slugToIDMap(ctx, models.TableNames.NotificationTargets, db)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			errs += fmt.Sprintf("%s\n", err)
		}
	}

	go getTargetSlugToIDMap()

	wg.Wait()

	if errs != "" {
		return nil, newErrUpdateNotificationPreferences(errs)
	}

	curr, err := GetNotificationPreferences(ctx, user.ID, ex, false)
	if err != nil {
		return nil, err
	}

	for _, p := range notificationPreferences {
		notificationTypeID, ok := typeSlugToID[p.NotificationType]
		if !ok {
			return nil,
				newErrUpdateNotificationPreferences(fmt.Sprintf("notificationType %s not found", p.NotificationType))
		}

		np := &models.NotificationPreference{
			UserID:               user.ID,
			NotificationTypeID:   notificationTypeID,
			NotificationTargetID: null.NewString("", false),
			Enabled:              p.Enabled,
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
					newErrUpdateNotificationPreferences(fmt.Sprintf("notificationTarget %s not found", t.Target))
			}

			np := &models.NotificationPreference{
				UserID:               user.ID,
				NotificationTypeID:   notificationTypeID,
				NotificationTargetID: null.NewString(notificationTargetID, true),
				Enabled:              t.Enabled,
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
