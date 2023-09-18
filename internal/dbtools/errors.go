package dbtools

import "errors"

// ErrUnknownRequestKind is returned a request kind is unknown
var ErrUnknownRequestKind = errors.New("request kind is unrecognized")
