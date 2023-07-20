package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"go.equinixmetal.net/governor-api/internal/models"
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
