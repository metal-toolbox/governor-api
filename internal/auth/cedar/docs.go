// Package cedar provides an optional Cedar-based authorization layer that
// authorizes callers from a given OIDC issuer via a local cedar-agent sidecar
// instead of by governor scopes. It talks to the sidecar over HTTP and is only
// consulted for issuers where it is explicitly enabled; otherwise the auth
// middleware is unchanged.
package cedar
