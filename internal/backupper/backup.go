package backupper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/jmoiron/sqlx"
	crdbmodels "github.com/metal-toolbox/governor-api/internal/models/crdb"
	psqlmodels "github.com/metal-toolbox/governor-api/internal/models/psql"
	"go.uber.org/zap"
)

// Backupper is responsible for backing up and restoring the system.
type Backupper struct {
	// Add fields and methods as needed
	driver DBDriver
	db     *sqlx.DB
	logger *zap.Logger
}

// Opt is a functional option for configuring the Backupper.
type Opt func(*Backupper)

// WithLogger sets the logger for the Backupper.
func WithLogger(logger *zap.Logger) Opt {
	return func(b *Backupper) {
		b.logger = logger
	}
}

// New creates a new Backupper instance.
func New(db *sqlx.DB, driver DBDriver, opts ...Opt) *Backupper {
	b := &Backupper{
		db:     db,
		logger: zap.NewNop(),
		driver: driver,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Backup creates a backup of the governor system.
func (b *Backupper) Backup(ctx context.Context, out io.Writer) error {
	var backup any

	switch b.driver {
	case DBDriverCRDB:
		bk, err := b.backupCRDB(ctx)
		if err != nil {
			return err
		}

		backup = bk
	case DBDriverPostgres:
		bk, err := b.backupPSQL(ctx)
		if err != nil {
			return err
		}

		backup = bk
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedDBDriver, b.driver)
	}

	j, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup: %w", err)
	}

	_, err = out.Write(j)
	if err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

// backupCRDB creates a backup of the governor system from CRDB
func (b *Backupper) backupCRDB(ctx context.Context) (*BackupCRDB, error) {
	var (
		thewholething = &BackupCRDB{}
		err           error
	)

	b.logger.Info("Starting backup of governor system")

	// Application-related data
	thewholething.ApplicationTypes, err = crdbmodels.ApplicationTypes(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup application types", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting applications types", zap.Int("count", len(thewholething.ApplicationTypes)))

	thewholething.Applications, err = crdbmodels.Applications(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup applications", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting applications", zap.Int("count", len(thewholething.Applications)))

	// Audit data
	thewholething.AuditEvents, err = crdbmodels.AuditEvents(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup audit events", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting audit events", zap.Int("count", len(thewholething.AuditEvents)))

	// Group-related data
	thewholething.GroupApplicationRequests, err = crdbmodels.GroupApplicationRequests().All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group application requests", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group application requests", zap.Int("count", len(thewholething.GroupApplicationRequests)))

	thewholething.GroupApplications, err = crdbmodels.GroupApplications(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group applications", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group applications", zap.Int("count", len(thewholething.GroupApplications)))

	thewholething.GroupHierarchies, err = crdbmodels.GroupHierarchies(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group hierarchies", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group hierarchies", zap.Int("count", len(thewholething.GroupHierarchies)))

	thewholething.GroupMembershipRequests, err = crdbmodels.GroupMembershipRequests(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group membership requests", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group membership requests", zap.Int("count", len(thewholething.GroupMembershipRequests)))

	thewholething.GroupMemberships, err = crdbmodels.GroupMemberships(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group memberships", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group memberships", zap.Int("count", len(thewholething.GroupMemberships)))

	thewholething.GroupOrganizations, err = crdbmodels.GroupOrganizations(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group organizations", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group organizations", zap.Int("count", len(thewholething.GroupOrganizations)))

	thewholething.Groups, err = crdbmodels.Groups(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup groups", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting groups", zap.Int("count", len(thewholething.Groups)))

	// Notification data
	thewholething.NotificationPreferences, err = crdbmodels.NotificationPreferences(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification preferences", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting notification preferences", zap.Int("count", len(thewholething.NotificationPreferences)))

	thewholething.NotificationTargets, err = crdbmodels.NotificationTargets(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification targets", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting notification targets", zap.Int("count", len(thewholething.NotificationTargets)))

	thewholething.NotificationTypes, err = crdbmodels.NotificationTypes(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification types", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting notification types", zap.Int("count", len(thewholething.NotificationTypes)))

	// Organization data
	thewholething.Organizations, err = crdbmodels.Organizations(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup organizations", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting organizations", zap.Int("count", len(thewholething.Organizations)))

	// User data
	thewholething.Users, err = crdbmodels.Users(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup users", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting users", zap.Int("count", len(thewholething.Users)))

	// Extension data
	thewholething.ExtensionResourceDefinitions, err = crdbmodels.ExtensionResourceDefinitions(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup extension resource definitions", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting extension resource definitions", zap.Int("count", len(thewholething.ExtensionResourceDefinitions)))

	thewholething.Extensions, err = crdbmodels.Extensions(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup extensions", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting extensions", zap.Int("count", len(thewholething.Extensions)))

	thewholething.SystemExtensionResources, err = crdbmodels.SystemExtensionResources(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup system extension resources", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting system extension resources", zap.Int("count", len(thewholething.SystemExtensionResources)))

	thewholething.UserExtensionResources, err = crdbmodels.UserExtensionResources(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup user extension resources", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting user extension resources", zap.Int("count", len(thewholething.UserExtensionResources)))

	b.logger.Info("Successfully completed backup of governor system")

	return thewholething, nil
}

func (b *Backupper) backupPSQL(ctx context.Context) (*BackupPSQL, error) {
	var (
		thewholething = &BackupPSQL{}
		err           error
	)

	b.logger.Info("Starting backup of governor system")

	// Application-related data
	thewholething.ApplicationTypes, err = psqlmodels.ApplicationTypes(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup application types", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting applications types", zap.Int("count", len(thewholething.ApplicationTypes)))

	thewholething.Applications, err = psqlmodels.Applications(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup applications", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting applications", zap.Int("count", len(thewholething.Applications)))

	// Audit data
	thewholething.AuditEvents, err = psqlmodels.AuditEvents(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup audit events", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting audit events", zap.Int("count", len(thewholething.AuditEvents)))

	// Group-related data
	thewholething.GroupApplicationRequests, err = psqlmodels.GroupApplicationRequests().All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group application requests", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group application requests", zap.Int("count", len(thewholething.GroupApplicationRequests)))

	thewholething.GroupApplications, err = psqlmodels.GroupApplications(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group applications", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group applications", zap.Int("count", len(thewholething.GroupApplications)))

	thewholething.GroupHierarchies, err = psqlmodels.GroupHierarchies(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group hierarchies", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group hierarchies", zap.Int("count", len(thewholething.GroupHierarchies)))

	thewholething.GroupMembershipRequests, err = psqlmodels.GroupMembershipRequests(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group membership requests", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group membership requests", zap.Int("count", len(thewholething.GroupMembershipRequests)))

	thewholething.GroupMemberships, err = psqlmodels.GroupMemberships(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group memberships", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group memberships", zap.Int("count", len(thewholething.GroupMemberships)))

	thewholething.GroupOrganizations, err = psqlmodels.GroupOrganizations(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group organizations", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting group organizations", zap.Int("count", len(thewholething.GroupOrganizations)))

	thewholething.Groups, err = psqlmodels.Groups(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup groups", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting groups", zap.Int("count", len(thewholething.Groups)))

	// Notification data
	thewholething.NotificationPreferences, err = psqlmodels.NotificationPreferences(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification preferences", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting notification preferences", zap.Int("count", len(thewholething.NotificationPreferences)))

	thewholething.NotificationTargets, err = psqlmodels.NotificationTargets(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification targets", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting notification targets", zap.Int("count", len(thewholething.NotificationTargets)))

	thewholething.NotificationTypes, err = psqlmodels.NotificationTypes(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification types", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting notification types", zap.Int("count", len(thewholething.NotificationTypes)))

	// Organization data
	thewholething.Organizations, err = psqlmodels.Organizations(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup organizations", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting organizations", zap.Int("count", len(thewholething.Organizations)))

	// User data
	thewholething.Users, err = psqlmodels.Users(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup users", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting users", zap.Int("count", len(thewholething.Users)))

	// Extension data
	thewholething.ExtensionResourceDefinitions, err = psqlmodels.ExtensionResourceDefinitions(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup extension resource definitions", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting extension resource definitions", zap.Int("count", len(thewholething.ExtensionResourceDefinitions)))

	thewholething.Extensions, err = psqlmodels.Extensions(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup extensions", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting extensions", zap.Int("count", len(thewholething.Extensions)))

	thewholething.SystemExtensionResources, err = psqlmodels.SystemExtensionResources(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup system extension resources", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting system extension resources", zap.Int("count", len(thewholething.SystemExtensionResources)))

	thewholething.UserExtensionResources, err = psqlmodels.UserExtensionResources(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup user extension resources", zap.Error(err))
		return nil, err
	}

	b.logger.Info("getting user extension resources", zap.Int("count", len(thewholething.UserExtensionResources)))

	b.logger.Info("Successfully completed backup of governor system")

	return thewholething, nil
}
