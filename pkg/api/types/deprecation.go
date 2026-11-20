package types

// ExternalIDDeprecationLink is the Link header value of a response that
// carries the deprecated external_id field. It points at the migration notes
// for moving from external_id to external_ids.
const ExternalIDDeprecationLink = `<https://github.com/metal-toolbox/governor-api/blob/main/docs/user-external-ids.md>; rel="deprecation"`
