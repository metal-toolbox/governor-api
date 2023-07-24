package dbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type UserNotificationPreferenceTarget struct {
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
}

type UserNotificationPreferenceTargets []*UserNotificationPreferenceTarget

type UserNotificationPreference struct {
	NotificationType string `json:"notification_type" boil:"notification_type"`
	Enabled          bool   `json:"enabled"`

	NotificationTargets UserNotificationPreferenceTargets `json:"notification_targets"`
}

type UserNotificationPreferences []*UserNotificationPreference

func RefreshNotificationDefaults(ctx context.Context, ex boil.ContextExecutor) (err error) {
	q := queries.Raw("REFRESH MATERIALIZED VIEW notification_defaults")
	_, err = q.ExecContext(ctx, ex)
	return
}

func GetNotificationPreferences(ctxx context.Context, uid string, db *sqlx.DB) (UserNotificationPreferences, error) {
	type preferencesQueryRecord struct {
		NotificationType        string          `boil:"notification_type"`
		NotificationTargetsJSON json.RawMessage `boil:"notification_targets"`
	}

	type preferencesQueryRecordNotificationTarget struct {
		Target  string `json:"f1"`
		Enabled bool   `json:"f2"`
	}

	ctx := boil.WithDebug(ctxx, true)

	records := []*preferencesQueryRecord{}
	q := models.NewQuery(
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
		qm.Select("nd.type_slug AS notification_type"),
		qm.Select("jsonb_agg((nd.target_slug, IFNULL(np.enabled, nd.default_enabled))) AS notification_targets"),
		qm.From("notification_defaults as nd"),
		qm.FullOuterJoin("np on (np.target_id = nd.target_id AND np.type_id = nd.type_id)"),
		qm.GroupBy("nd.type_slug"),
	)

	if err := q.Bind(ctx, db, &records); err != nil {
		return nil, err
	}

	errs := ""
	wg := &sync.WaitGroup{}
	preferences := make(UserNotificationPreferences, len(records))
	for i, r := range records {
		wg.Add(1)
		go func(i int, r *preferencesQueryRecord) {
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

		}(i, r)
	}

	wg.Wait()
	if errs != "" {
		return nil, fmt.Errorf(errs)
	}

	return preferences, nil
}

func slugToIDMap(ctx context.Context, table string, db *sqlx.DB) (slugIdMap map[string]string, err error) {
	rec := &struct {
		Map json.RawMessage `boil:"map"`
	}{}

	q := models.NewQuery(
		qm.Select(
			"1 as tmp",
			"jsonb_object_agg(slug, id) as map",
		),
		qm.From(table),
		qm.GroupBy("1"),
	)

	if err = q.Bind(ctx, db, rec); err != nil {
		return
	}

	err = json.Unmarshal(rec.Map, &slugIdMap)
	return
}

func CreateOrUpdateNotificationPreferences(
	ctx context.Context, uid string,
	p UserNotificationPreferences,
	ex boil.ContextExecutor,
	db *sqlx.DB,
) error {
	typeSlugToID := map[string]string{}
	targetSlugToID := map[string]string{}
	errs := ""
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		typeSlugToID, err = slugToIDMap(ctx, models.TableNames.NotificationTypes, db)
		if err != nil {
			errs += fmt.Sprintf("%s\n", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		targetSlugToID, err = slugToIDMap(ctx, models.TableNames.NotificationTargets, db)
		if err != nil {
			errs += fmt.Sprintf("%s\n", err)
		}
	}()

	wg.Wait()
	if errs != "" {
		return fmt.Errorf(errs)
	}

	fmt.Println(typeSlugToID)

	valuesarg := []interface{}{uid}
	valuessql := []string{}
	placeholderCounter := 2

	for _, preference := range p {
		typeID, ok := typeSlugToID[preference.NotificationType]
		if !ok {
			return fmt.Errorf("notification type %s does not exist", preference.NotificationType)
		}

		// type preferences
		valuessql = append(
			valuessql,
			fmt.Sprintf(
				"($1, $%d, NULL, $%d)",
				placeholderCounter,
				placeholderCounter+1,
			),
		)

		placeholderCounter += 2

		valuesarg = append(valuesarg, typeID)
		valuesarg = append(valuesarg, preference.Enabled)

		// target preferences
		for _, notificationTarget := range preference.NotificationTargets {
			targetID, ok := targetSlugToID[notificationTarget.Target]
			if !ok {
				return fmt.Errorf("notification target %s does not exist", notificationTarget.Target)
			}

			valuessql = append(
				valuessql,
				fmt.Sprintf(
					"($1, $%d, $%d, $%d)",
					placeholderCounter,
					placeholderCounter+1,
					placeholderCounter+2,
				),
			)
			placeholderCounter += 3

			valuesarg = append(valuesarg, typeID)
			valuesarg = append(valuesarg, targetID)
			valuesarg = append(valuesarg, notificationTarget.Enabled)
		}
	}

	rawInsertQuery := fmt.Sprintf(
		"INSERT INTO \"%s\" %s\n VALUES\n%s\n%s\n;",
		models.TableNames.NotificationPreferences,
		fmt.Sprintf(
			"(%s, %s, %s, %s)",
			models.NotificationPreferenceColumns.UserID,
			models.NotificationPreferenceColumns.NotificationTypeID,
			models.NotificationPreferenceColumns.NotificationTargetID,
			models.NotificationPreferenceColumns.Enabled,
		),
		strings.Join(valuessql, ",\n"),
		fmt.Sprintf(
			"ON CONFLICT (%s, %s, %s)\nDO UPDATE SET %s = excluded.%s",
			models.NotificationPreferenceColumns.UserID,
			models.NotificationPreferenceColumns.NotificationTypeID,
			models.NotificationPreferenceColumns.NotificationTargetIDNullString,
			models.NotificationPreferenceColumns.Enabled,
			models.NotificationPreferenceColumns.Enabled,
		),
	)

	q := queries.Raw(rawInsertQuery, valuesarg...)
	_, err := q.ExecContext(ctx, ex)
	return err
}
