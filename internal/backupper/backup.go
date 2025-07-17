package backupper

import (
	"context"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/jmoiron/sqlx"
	models "github.com/metal-toolbox/governor-api/internal/models/crdb"
	"go.uber.org/zap"
)

// BackupCRDB represents a backup for the CRDB database.
type BackupCRDB struct {
	ApplicationTypes models.ApplicationTypeSlice `json:"application_types"`
	Applications     models.ApplicationSlice     `json:"applications"`
	AuditEvents      models.AuditEventSlice      `json:"audit_events"`

	GroupApplicationRequests models.GroupApplicationRequestSlice `json:"group_application_requests"`
	GroupApplications        models.GroupApplicationSlice        `json:"group_applications"`
	GroupHierarchies         models.GroupHierarchySlice          `json:"group_hierarchies"`
	GroupMembershipRequests  models.GroupMembershipRequestSlice  `json:"group_membership_requests"`
	GroupMemberships         models.GroupMembershipSlice         `json:"group_memberships"`
	GroupOrganizations       models.GroupOrganizationSlice       `json:"group_organizations"`
	Groups                   models.GroupSlice                   `json:"groups"`

	NotificationPreferences models.NotificationPreferenceSlice `json:"notification_preferences"`
	NotificationTargets     models.NotificationTargetSlice     `json:"notification_targets"`
	NotificationTypes       models.NotificationTypeSlice       `json:"notification_types"`

	Organizations models.OrganizationSlice `json:"organizations"`

	Users models.UserSlice `json:"users"`

	ExtensionResourceDefinitions models.ExtensionResourceDefinitionSlice `json:"extension_resource_definitions"`
	Extensions                   models.ExtensionSlice                   `json:"extensions"`
	SystemExtensionResources     models.SystemExtensionResourceSlice     `json:"system_extension_resources"`
	UserExtensionResources       models.UserExtensionResourceSlice       `json:"user_extension_resources"`
}

// Backupper is responsible for backing up and restoring the system.
type Backupper struct {
	// Add fields and methods as needed
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

// NewBackupper creates a new Backupper instance.
func NewBackupper(db *sqlx.DB, opts ...Opt) *Backupper {
	b := &Backupper{
		db:     db,
		logger: zap.NewNop(),
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// BackupCRDB creates a backup of the governor system from CRDB
func (b *Backupper) BackupCRDB(ctx context.Context) (*BackupCRDB, error) {
	var (
		thewholething = &BackupCRDB{}
		err           error
	)

	b.logger.Info("Starting backup of governor system")

	// Application-related data
	thewholething.ApplicationTypes, err = models.ApplicationTypes(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup application types", zap.Error(err))
		return nil, err
	}

	thewholething.Applications, err = models.Applications(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup applications", zap.Error(err))
		return nil, err
	}

	// Audit data
	thewholething.AuditEvents, err = models.AuditEvents(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup audit events", zap.Error(err))
		return nil, err
	}

	// Group-related data
	thewholething.GroupApplicationRequests, err = models.GroupApplicationRequests().All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group application requests", zap.Error(err))
		return nil, err
	}

	thewholething.GroupApplications, err = models.GroupApplications(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group applications", zap.Error(err))
		return nil, err
	}

	thewholething.GroupHierarchies, err = models.GroupHierarchies(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group hierarchies", zap.Error(err))
		return nil, err
	}

	thewholething.GroupMembershipRequests, err = models.GroupMembershipRequests(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group membership requests", zap.Error(err))
		return nil, err
	}

	thewholething.GroupMemberships, err = models.GroupMemberships(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group memberships", zap.Error(err))
		return nil, err
	}

	thewholething.GroupOrganizations, err = models.GroupOrganizations(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup group organizations", zap.Error(err))
		return nil, err
	}

	thewholething.Groups, err = models.Groups(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup groups", zap.Error(err))
		return nil, err
	}

	// Notification data
	thewholething.NotificationPreferences, err = models.NotificationPreferences(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification preferences", zap.Error(err))
		return nil, err
	}

	thewholething.NotificationTargets, err = models.NotificationTargets(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification targets", zap.Error(err))
		return nil, err
	}

	thewholething.NotificationTypes, err = models.NotificationTypes(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup notification types", zap.Error(err))
		return nil, err
	}

	// Organization data
	thewholething.Organizations, err = models.Organizations(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup organizations", zap.Error(err))
		return nil, err
	}

	// User data
	thewholething.Users, err = models.Users(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup users", zap.Error(err))
		return nil, err
	}

	// Extension data
	thewholething.ExtensionResourceDefinitions, err = models.ExtensionResourceDefinitions(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup extension resource definitions", zap.Error(err))
		return nil, err
	}

	thewholething.Extensions, err = models.Extensions(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup extensions", zap.Error(err))
		return nil, err
	}

	thewholething.SystemExtensionResources, err = models.SystemExtensionResources(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup system extension resources", zap.Error(err))
		return nil, err
	}

	thewholething.UserExtensionResources, err = models.UserExtensionResources(qm.WithDeleted()).All(ctx, b.db)
	if err != nil {
		b.logger.Error("Failed to backup user extension resources", zap.Error(err))
		return nil, err
	}

	b.logger.Info("Successfully completed backup of governor system")

	return thewholething, nil
}
