package dbtools

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/jmoiron/sqlx"
	dbm "github.com/metal-toolbox/governor-api/db"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/suite"
)

// NotificationPreferencesTestSuite is a test suite to run unit tests on
// all notification preferences db tests
type NotificationPreferencesTestSuite struct {
	suite.Suite

	db      *sql.DB
	uid     string
	auditID string

	trueptr  *bool
	falseptr *bool
}

func (s *NotificationPreferencesTestSuite) seedTestDB() error {
	testData := []string{
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000001', NULL, 'User1', 'user1@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'active');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000002', NULL, 'User2', 'user2@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'active');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000003', NULL, 'User3', 'user3@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'active');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000004', NULL, 'User4', 'user4@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'active');`,
	}
	for _, q := range testData {
		_, err := s.db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *NotificationPreferencesTestSuite) SetupSuite() {
	ts, err := testserver.NewTestServer()
	if err != nil {
		panic(err)
	}

	s.db, err = sql.Open("postgres", ts.PGURL().String())
	if err != nil {
		panic(err)
	}

	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Up(s.db, "migrations"); err != nil {
		panic("migration failed - could not set up test db")
	}

	if err := s.seedTestDB(); err != nil {
		panic("db setup failed - could not seed test db")
	}

	s.uid = "00000001-0000-0000-0000-000000000001"
	s.auditID = "10000001-0000-0000-0000-000000000001"

	s.trueptr = new(bool)
	s.falseptr = new(bool)
	*s.trueptr = true
	*s.falseptr = false
}

func (s *NotificationPreferencesTestSuite) TestNotificationDefaults() {
	tests := []struct {
		name         string
		q            string
		want         UserNotificationPreferences
		wantQueryErr bool
		wantFnErr    bool
	}{
		{
			name: "insert notification type",
			q:    `INSERT INTO notification_types (id, name, slug, description, default_enabled) VALUES ('00000000-0000-0000-0000-000000000001', 'Alert', 'alert', 'aleeeeeeeert', 'false')`,
			want: UserNotificationPreferences{{
				NotificationType:    "alert",
				Enabled:             s.falseptr,
				NotificationTargets: UserNotificationPreferenceTargets{},
			}},
			wantQueryErr: false,
			wantFnErr:    false,
		},
		{
			name:         "insert notification type duplication",
			q:            `INSERT INTO notification_types (id, name, slug, description, default_enabled) VALUES ('00000000-0000-0000-0000-000000000001', 'Alert', 'alert', 'aleeeeeeeert', 'false')`,
			want:         UserNotificationPreferences{nil},
			wantQueryErr: true,
			wantFnErr:    false,
		},
		{
			name: "update notification type defaults",
			q:    `UPDATE notification_types SET default_enabled = 'true' WHERE id = '00000000-0000-0000-0000-000000000001'`,
			want: UserNotificationPreferences{{
				NotificationType:    "alert",
				Enabled:             s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{},
			}},
			wantQueryErr: false,
			wantFnErr:    false,
		},
		{
			name: "insert notification target",
			q:    `INSERT INTO notification_targets (id, name, slug, description, default_enabled) VALUES ('00000000-0000-0000-0000-000000000001', 'Slack', 'slack', 'nice', 'false')`,
			want: UserNotificationPreferences{{
				NotificationType: "alert",
				Enabled:          s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{{
					Target:  "slack",
					Enabled: s.falseptr,
				}},
			}},
			wantQueryErr: false,
			wantFnErr:    false,
		},
		{
			name:         "insert notification target duplication",
			q:            `INSERT INTO notification_targets (id, name, slug, description, default_enabled) VALUES ('00000000-0000-0000-0000-000000000001', 'Slack', 'slack', 'nice', 'true')`,
			want:         UserNotificationPreferences{nil},
			wantQueryErr: true,
			wantFnErr:    false,
		},
		{
			name: "update notification target defaults",
			q:    `UPDATE notification_targets SET default_enabled = 'true' WHERE id = '00000000-0000-0000-0000-000000000001'`,
			want: UserNotificationPreferences{{
				NotificationType: "alert",
				Enabled:          s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{{
					Target:  "slack",
					Enabled: s.trueptr,
				}},
			}},
			wantQueryErr: false,
			wantFnErr:    false,
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			_, err := s.db.Query(tc.q)

			if tc.wantQueryErr {
				s.Assert().Error(err)
				return
			}

			ctx := context.TODO()

			if err := RefreshNotificationDefaults(ctx, s.db); err != nil {
				panic(err)
			}

			actual, err := GetNotificationPreferences(ctx, s.uid, s.db, true)

			if tc.wantFnErr {
				s.Assert().Error(err)
				return
			}

			s.Assert().NoError(err)
			s.Assert().Equal(len(tc.want), len(actual))

			for i, p := range actual {
				s.Assert().Equal(*tc.want[i], *p)
			}
		})
	}
}

func (s *NotificationPreferencesTestSuite) TestNotificationPreferences() {
	u := &models.User{
		ID: s.uid,
	}

	sqlxdb := sqlx.NewDb(s.db, "postgres")

	tests := []struct {
		name                string
		action              func() error
		wantWithDefaults    UserNotificationPreferences
		wantWithoutDefaults UserNotificationPreferences
		wantErr             bool
	}{
		{
			name:   "get notification preferences",
			action: nil,
			wantWithDefaults: UserNotificationPreferences{{
				NotificationType: "alert",
				Enabled:          s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{{
					Target:  "slack",
					Enabled: s.trueptr,
				}},
			}},
			wantWithoutDefaults: UserNotificationPreferences{},
			wantErr:             false,
		},
		{
			name: "user updates notification type preferences",
			action: func() error {
				_, e := CreateOrUpdateNotificationPreferences(
					context.TODO(), u,
					UserNotificationPreferences{{
						NotificationType: "alert",
						Enabled:          s.trueptr,
					}},
					sqlxdb, s.auditID, u,
				)

				return e
			},
			wantWithDefaults: UserNotificationPreferences{{
				NotificationType: "alert",
				Enabled:          s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{{
					Target:  "slack",
					Enabled: s.trueptr,
				}},
			}},
			wantWithoutDefaults: UserNotificationPreferences{{
				NotificationType:    "alert",
				Enabled:             s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{},
			}},
			wantErr: false,
		},
		{
			name: "user updates notification target preferences",
			action: func() error {
				_, e := CreateOrUpdateNotificationPreferences(
					context.TODO(), u,
					UserNotificationPreferences{{
						NotificationType: "alert",
						Enabled:          s.trueptr,
						NotificationTargets: UserNotificationPreferenceTargets{{
							Target:  "slack",
							Enabled: s.falseptr,
						}},
					}},
					sqlxdb, s.auditID, u,
				)

				return e
			},
			wantWithDefaults: UserNotificationPreferences{{
				NotificationType: "alert",
				Enabled:          s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{{
					Target:  "slack",
					Enabled: s.falseptr,
				}},
			}},
			wantWithoutDefaults: UserNotificationPreferences{{
				NotificationType: "alert",
				Enabled:          s.trueptr,
				NotificationTargets: UserNotificationPreferenceTargets{{
					Target:  "slack",
					Enabled: s.falseptr,
				}},
			}},
			wantErr: false,
		},
		{
			name: "user updates notification preferences with invalid type",
			action: func() error {
				_, e := CreateOrUpdateNotificationPreferences(
					context.TODO(), u,
					UserNotificationPreferences{{
						NotificationType: "invalid-type",
						Enabled:          s.trueptr,
					}},
					sqlxdb, s.auditID, u,
				)

				return e
			},
			wantWithDefaults:    UserNotificationPreferences{nil},
			wantWithoutDefaults: UserNotificationPreferences{nil},
			wantErr:             true,
		},
		{
			name: "user updates notification preferences with invalid target",
			action: func() error {
				_, e := CreateOrUpdateNotificationPreferences(
					context.TODO(), u,
					UserNotificationPreferences{{
						NotificationType: "alert",
						Enabled:          s.trueptr,
						NotificationTargets: UserNotificationPreferenceTargets{{
							Target:  "invalid-target",
							Enabled: s.falseptr,
						}},
					}},
					sqlxdb, s.auditID, u,
				)

				return e
			},
			wantWithDefaults:    UserNotificationPreferences{nil},
			wantWithoutDefaults: UserNotificationPreferences{nil},
			wantErr:             true,
		},
		{
			name: "user updates notification preferences with empty target enabled value",
			action: func() error {
				_, e := CreateOrUpdateNotificationPreferences(
					context.TODO(), u,
					UserNotificationPreferences{{
						NotificationType: "alert",
						Enabled:          s.trueptr,
						NotificationTargets: UserNotificationPreferenceTargets{{
							Target: "slack",
						}},
					}},
					sqlxdb, s.auditID, u,
				)

				return e
			},
			wantWithDefaults:    UserNotificationPreferences{nil},
			wantWithoutDefaults: UserNotificationPreferences{nil},
			wantErr:             true,
		},
		{
			name: "user updates notification preferences with empty type enabled value",
			action: func() error {
				_, e := CreateOrUpdateNotificationPreferences(
					context.TODO(), u,
					UserNotificationPreferences{{
						NotificationType: "alert",
						NotificationTargets: UserNotificationPreferenceTargets{{
							Target:  "slack",
							Enabled: s.trueptr,
						}},
					}},
					sqlxdb, s.auditID, u,
				)

				return e
			},
			wantWithDefaults:    UserNotificationPreferences{nil},
			wantWithoutDefaults: UserNotificationPreferences{nil},
			wantErr:             true,
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			if tc.action != nil {
				err := tc.action()

				if tc.wantErr {
					s.Assert().Error(err)
					return
				}

				s.Assert().NoError(err)
			}

			ctx := context.TODO()

			actualWithDefaults, err := GetNotificationPreferences(ctx, s.uid, s.db, true)
			s.Assert().NoError(err)

			actualWithoutDefaults, err := GetNotificationPreferences(ctx, s.uid, s.db, false)
			s.Assert().NoError(err)

			s.Assert().Equal(len(tc.wantWithDefaults), len(actualWithDefaults))
			s.Assert().Equal(len(tc.wantWithoutDefaults), len(actualWithoutDefaults))

			for i, p := range actualWithDefaults {
				s.Assert().Equal(*tc.wantWithDefaults[i], *p)
			}

			for i, p := range actualWithoutDefaults {
				s.Assert().Equal(*tc.wantWithoutDefaults[i], *p)
			}
		})
	}
}

func TestNotificationPreferencesTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationPreferencesTestSuite))
}
