package backupper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"go.uber.org/zap"
)

// Restore restores the database from a backup
func (b *Backupper) Restore(ctx context.Context, reader io.Reader) error {
	databytes, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	var restorationGroups []restorationGroup

	switch b.driver {
	case DBDriverPostgres:
		var data BackupPSQL

		if err := json.Unmarshal(databytes, &data); err != nil {
			return err
		}

		restorationGroups = []restorationGroup{
			{name: "application types", data: toInsertable(data.ApplicationTypes)},
			{name: "groups", data: toInsertable(data.Groups)},
			{name: "users", data: toInsertable(data.Users)},
			{name: "extensions", data: toInsertable(data.Extensions)},
			{name: "notification targets", data: toInsertable(data.NotificationTargets)},
			{name: "notification types", data: toInsertable(data.NotificationTypes)},
			{name: "organizations", data: toInsertable(data.Organizations)},
			{name: "applications", data: toInsertable(data.Applications)},
			{name: "audit events", data: toInsertable(data.AuditEvents)},
			{name: "extension resource definitions", data: toInsertable(data.ExtensionResourceDefinitions)},
			{name: "group application requests", data: toInsertable(data.GroupApplicationRequests)},
			{name: "group applications", data: toInsertable(data.GroupApplications)},
			{name: "group hierarchies", data: toInsertable(data.GroupHierarchies)},
			{name: "group membership requests", data: toInsertable(data.GroupMembershipRequests)},
			{name: "group memberships", data: toInsertable(data.GroupMemberships)},
			{name: "group organizations", data: toInsertable(data.GroupOrganizations)},
			{name: "notification preferences", data: toInsertable(data.NotificationPreferences)},
			{name: "system extension resources", data: toInsertable(data.SystemExtensionResources)},
			{name: "user extension resources", data: toInsertable(data.UserExtensionResources)},
		}

	case DBDriverCRDB:
		var data BackupCRDB

		if err := json.Unmarshal(databytes, &data); err != nil {
			return err
		}

		restorationGroups = []restorationGroup{
			{name: "application types", data: toInsertable(data.ApplicationTypes)},
			{name: "groups", data: toInsertable(data.Groups)},
			{name: "users", data: toInsertable(data.Users)},
			{name: "extensions", data: toInsertable(data.Extensions)},
			{name: "notification targets", data: toInsertable(data.NotificationTargets)},
			{name: "notification types", data: toInsertable(data.NotificationTypes)},
			{name: "organizations", data: toInsertable(data.Organizations)},
			{name: "applications", data: toInsertable(data.Applications)},
			{name: "audit events", data: toInsertable(data.AuditEvents)},
			{name: "extension resource definitions", data: toInsertable(data.ExtensionResourceDefinitions)},
			{name: "group application requests", data: toInsertable(data.GroupApplicationRequests)},
			{name: "group applications", data: toInsertable(data.GroupApplications)},
			{name: "group hierarchies", data: toInsertable(data.GroupHierarchies)},
			{name: "group membership requests", data: toInsertable(data.GroupMembershipRequests)},
			{name: "group memberships", data: toInsertable(data.GroupMemberships)},
			{name: "group organizations", data: toInsertable(data.GroupOrganizations)},
			{name: "notification preferences", data: toInsertable(data.NotificationPreferences)},
			{name: "system extension resources", data: toInsertable(data.SystemExtensionResources)},
			{name: "user extension resources", data: toInsertable(data.UserExtensionResources)},
		}

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedDBDriver, b.driver)
	}

	return b.restore(ctx, restorationGroups)
}

func insertData(ctx context.Context, tx *sqlx.Tx, items []insertable) error {
	for _, item := range items {
		if err := item.Insert(ctx, tx, boil.Infer()); err != nil {
			if err := tx.Rollback(); err != nil {
				return err
			}

			return err
		}
	}

	return nil
}

func toInsertable[T insertable](items []T) []insertable {
	result := make([]insertable, len(items))
	for i, item := range items {
		result[i] = item
	}

	return result
}

func (b *Backupper) restore(ctx context.Context, groups []restorationGroup) error {
	tx, err := b.db.BeginTxx(ctx, nil)
	if err != nil {
		b.logger.Error("Failed to begin transaction", zap.Error(err))
		return err
	}

	b.logger.Info("Starting restore of governor system")

	for _, data := range groups {
		if err := insertData(ctx, tx, data.data); err != nil {
			b.logger.Error("Failed to restore "+data.name, zap.Error(err))
			return err
		}

		b.logger.Info("Restored "+data.name, zap.Int("count", len(data.data)))
	}

	if err := dbtools.RefreshNotificationDefaults(ctx, b.db); err != nil {
		b.logger.Error("Failed to refresh notification defaults", zap.Error(err))

		if err := tx.Rollback(); err != nil {
			b.logger.Error("Failed to rollback transaction", zap.Error(err))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		b.logger.Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}
