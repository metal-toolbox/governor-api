# User external IDs

A governor user can have one identity at each identity provider (IdP) that
governor trusts. Each identity is an `ExternalID` object: the issuer URL of
the provider and the provider's ID for the user.

```json
{ "iss": "https://sso.example.com/oauth2/default", "id": "00u1abcd2EFGH3ijk4x7" }
```

The users API returns these identities in the `external_ids` array. The single
`external_id` string field is deprecated and will be removed in the next
release. Every response that still carries `external_id` sends these headers:

```text
Deprecation: true
Link: <https://github.com/metal-toolbox/governor-api/blob/main/docs/user-external-ids.md>; rel="deprecation"
```

## What `external_id` returns

The value of `external_id` is the user's ID at the primary identity provider:
the first `oidc` entry in the governor config whose `identity.enabled` is
`true`. It is `null` when the user has no identity at that provider, or when no
provider sets `identity.enabled`. Keep the provider that issued the existing
user IDs first in the `oidc` list, so that `external_id` keeps returning those
IDs.

## Moving from `external_id` to `external_ids`

### Reading users

Read `external_ids` and pick the entry whose `iss` is the provider you work
with, in place of reading `external_id`. The array is always present, and is
empty when the user has no identities.

### Finding a user by external ID

Add the `iss` query parameter to the `external_id` filter:

```text
GET /api/v1alpha1/users?external_id=00u1abcd2EFGH3ijk4x7&iss=https%3A%2F%2Fsso.example.com%2Foauth2%2Fdefault
```

`external_id` without `iss` matches the ID at any provider, and is deprecated.
`iss` alone returns the users that have an identity at that provider. The same
parameters work on `GET /api/v1beta1/users`. An `iss` that is not a configured
issuer returns `422`.

Go clients can call `Client.UserByExternalID(ctx, iss, externalID)` from
`pkg/client`. It returns `ErrUserNotFound` when no user matches.

### Creating and updating users

Send `external_ids` on `POST /api/v1alpha1/users` and
`PUT /api/v1alpha1/users/:id`:

```json
{
  "email": "ada@example.com",
  "name": "Ada Lovelace",
  "external_ids": [
    { "iss": "https://sso.example.com/oauth2/default", "id": "00u1abcd2EFGH3ijk4x7" }
  ]
}
```

- Each `iss` must be the issuer of an `oidc` entry with `identity.enabled: true`
  and may appear only once. A request may set at most 10 identities.
- On update, `external_ids` replaces the user's full set; an empty array
  removes them all. Leave the field out to keep the identities unchanged.
  A record at an issuer whose `identity.enabled` was turned off may be sent
  back unchanged, so a full-set update keeps it; it cannot be added or changed.
- A pair that is already linked to another user returns `409`.

Until it is removed, a request that sends `external_id` without `external_ids`
sets the user's identity at the primary identity provider. A request with both
fields returns `422 use either external_id or external_ids, not both`. When no
provider sets `identity.enabled`, `external_id` returns `422 external_id needs
an oidc provider with identity.enabled; use external_ids`.

## Signing in through a new provider

Each `oidc` entry has an `identity` block:

| Key | Default | Meaning |
|---|---|---|
| `identity.enabled` | `false` | Tokens from this provider resolve to governor users. When `false`, a sign-in through this provider returns `401 user not registered`, also for a user who already has an identity at its issuer. |
| `identity.autoLink` | `false` | On the first sign-in through this provider, link the identity to the existing active user with the same email. Used only when `identity.enabled` is `true`. |
| `identity.autoRegister` | `true` | On the first sign-in through this provider, create a user when no user matches. Used only when `identity.enabled` is `true`. |

At least one `oidc` entry must set `identity.enabled: true`. A user cannot
change their own identities through `PUT /api/v1alpha1/user`.
