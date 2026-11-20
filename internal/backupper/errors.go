package backupper

import "errors"

var (
	// ErrUnsupportedDBDriver is returned when an unsupported database driver is used.
	ErrUnsupportedDBDriver = errors.New("unsupported database driver")
	// ErrBackupWithoutExternalIDs is returned by a PostgreSQL restore of a backup
	// with no user_external_ids key. Such a backup was written by a release that
	// kept user identities in users.external_id, which this release no longer
	// has, so its users would restore without an identity. Restore restores
	// nothing in that case.
	ErrBackupWithoutExternalIDs = errors.New("backup has no user_external_ids: it was written by a release before user identities moved to user_external_ids; restore it with that release, then upgrade")
)
