package v1alpha1

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

func defaultNotificationPreferences(ctx context.Context, db *sqlx.DB) (UserNotificationPreferences, error) {
	var notificationTypes models.NotificationTypeSlice
	var notificationTargets models.NotificationTargetSlice
	errs := ""
	queryWg := &sync.WaitGroup{}

	queryWg.Add(1)
	func(nt *models.NotificationTypeSlice) {
		defer queryWg.Done()
		var err error
		*nt, err = models.NotificationTypes().All(ctx, db)
		if err != nil {
			errs += fmt.Sprintf("%s\n", err.Error())
		}
	}(&notificationTypes)

	queryWg.Add(1)
	func(nt *models.NotificationTargetSlice) {
		defer queryWg.Done()
		var err error
		*nt, err = models.NotificationTargets().All(ctx, db)
		if err != nil {
			errs += fmt.Sprintf("%s\n", err.Error())
		}
	}(&notificationTargets)

	queryWg.Wait()
	if errs != "" {
		return nil, fmt.Errorf(errs)
	}

	targets := make(UserNotificationPreferenceTargets, len(notificationTargets))
	for i, ntarget := range notificationTargets {
		t := &UserNotificationPreferenceTarget{
			Target:  ntarget.Slug,
			Enabled: ntarget.DefaultEnabled,
		}
		targets[i] = t
	}

	preferences := make(UserNotificationPreferences, len(notificationTypes))
	for i, ntype := range notificationTypes {
		p := new(UserNotificationPreference)
		p.NotificationType = ntype.Slug
		p.NotificationTargets = targets
		preferences[i] = p
	}

	return preferences, nil
}

func getNotificationPreferences(ctx context.Context, uid string, db *sqlx.DB) (UserNotificationPreferences, error) {
	type preferencesQueryRecord struct {
		NotificationType        string          `boil:"notification_type"`
		NotificationTargetsJSON json.RawMessage `boil:"notification_targets"`
	}

	type preferencesQueryRecordNotificationTarget struct {
		Target  string `json:"f1"`
		Enabled bool   `json:"f2"`
	}

	records := []*preferencesQueryRecord{}
	q := models.NewQuery(
		qm.Select("notification_types.slug AS notification_type"),
		qm.Select("jsonb_agg((notification_targets.slug, enabled)) AS notification_targets"),
		qm.From(models.TableNames.NotificationPreferences),
		qm.LeftOuterJoin("notification_targets on notification_target_id = notification_targets.id"),
		qm.LeftOuterJoin("notification_types on notification_type_id = notification_types.id"),
		qm.Where("user_id = ?", uid),
		qm.GroupBy("notification_types.slug"),
	)

	if err := q.Bind(ctx, db, &records); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return defaultNotificationPreferences(ctx, db)
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

func createOrUpdateNotificationPreferences(
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

	valuesarg := []interface{}{}
	valuessql := []string{}
	placeholderCounter := 1

	for _, preference := range p {
		typeID, ok := typeSlugToID[preference.NotificationType]
		if !ok {
			return fmt.Errorf("notification type %s does not exist", preference.NotificationType)
		}

		// type preferences
		valuessql = append(
			valuessql,
			fmt.Sprintf(
				"('%s', $%d, NULL, $%d)",
				uid,
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
					"('%s', $%d, $%d, $%d)",
					uid,
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
