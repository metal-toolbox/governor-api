package eventbus

import "errors"

// ErrEmptyEvent is returned when an empty event is passed
var ErrEmptyEvent = errors.New("event is empty")
