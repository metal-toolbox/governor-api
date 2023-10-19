package client

import "errors"

var (
	// ErrRequestNonSuccess is returned when a call to the governor API returns a non-success status
	ErrRequestNonSuccess = errors.New("got a non-success response from governor")

	// ErrGroupNotFound is returned when a group is not found
	ErrGroupNotFound = errors.New("group not found")

	// ErrMissingGroupID is returned when a missing or bad group id is passed to a request
	ErrMissingGroupID = errors.New("missing group id in request")

	// ErrMissingOrganizationID is returned when a missing or bad organization id is passed to a request
	ErrMissingOrganizationID = errors.New("missing organization id in request")

	// ErrMissingApplicationID is returned when a missing or bad application id is passed to a request
	ErrMissingApplicationID = errors.New("missing application id in request")

	// ErrMissingApplicationTypeID is returned when a missing or bad application_type id is passed to a request
	ErrMissingApplicationTypeID = errors.New("missing application_type id in request")

	// ErrMissingUserID is returned when a missing or bad user id is passed to a request
	ErrMissingUserID = errors.New("missing user id in request")

	// ErrMissingRequestID is returned when a missing or bad request id is passed to a request
	ErrMissingRequestID = errors.New("missing request id in request")

	// ErrNilUserRequest is returned when a nil user body is passed to a request
	ErrNilUserRequest = errors.New("nil user request")

	// ErrNilGroupRequest is returned when a nil group body is passed to a request
	ErrNilGroupRequest = errors.New("nil group request")

	// ErrUserNotFound is returned when a user is expected to be returned but instead is not
	ErrUserNotFound = errors.New("user not found")

	// ErrNotificationTypeNotFound is returned when a notification type is not found
	ErrNotificationTypeNotFound = errors.New("notification type not found")

	// ErrMissingNotificationTypeID is returned when a a missing or bad notification type ID is passed to a request
	ErrMissingNotificationTypeID = errors.New("missing notification type id in request")

	// ErrNotificationTargetNotFound is returned when a notification target is not found
	ErrNotificationTargetNotFound = errors.New("notification target not found")

	// ErrMissingNotificationTargetID is returned when a a missing or bad notification target ID is passed to a request
	ErrMissingNotificationTargetID = errors.New("missing notification target id in request")

	// ErrMissingExtensionIDOrSlug is returned when a missing or bad extension ID is passed to a request
	ErrMissingExtensionIDOrSlug = errors.New("missing extension id or slug in request")

	// ErrMissingERDIDOrSlug is returned when a a missing or bad extension resource definition ID is passed to a request
	ErrMissingERDIDOrSlug = errors.New("missing ERD id or slug in request")

	// ErrMissingResourceID is returned when a a missing or bad resource ID is passed to a request
	ErrMissingResourceID = errors.New("missing resource id in request")
)
