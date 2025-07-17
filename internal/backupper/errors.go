package backupper

import "errors"

// ErrUnsupportedDBDriver is returned when an unsupported database driver is used.
var ErrUnsupportedDBDriver = errors.New("unsupported database driver")
