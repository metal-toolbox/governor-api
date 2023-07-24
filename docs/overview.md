# Governor Design

The Governor ecosystem provides cohesive access management across a variety of services.  Primarily, it manages groups and users in an IDP, along with roles and groups in integrated services. It provides a web interface for users to login, request group enrollments, creates and manage groups as well as application integrations. It provides self service identity and access management with built-in approvals and auditing. For the Governor ecosystem, users are primarily meant to be human identities. Groups are collections of human identities and applications are any third party integrations that can have access control managed by a group. Some examples of application integrations are managing groups in an external IDP or managing Github team membership.

Governor aims to be a loosely coupled system, with identity data managed centrally, but integrations implemented as distributed systems called "addons".

![governor diagram](/governor.svg)

## Governor API

The Governor API is the datastore and the source of truth for the Governor ecosystem. It doesn't integrate directly with any downstream system but mearly manages IAM data and emits events when this data changes. The API can be leveraged directly or can be used to service the UI. The Governor API provides a versioned REST API.

Certain core functionality is provided by the Governor API model, any additions to this or expansion in scope should be carefully considered. A simple datastore that emits events is easier to reason about and easier to separate concerns than one with tight integrations to external services. Integrations "leakage" or scope shifting should be avoided.

## Addons and Events

Addons are the primary means to integrate external systems into the Governor ecosystem. The Governor API will emit events when managed resources change, and those events can be used to trigger addons to make changes to integrated services. The events emitted by the Governor API are not expected to include the necessary data for completing integrations, but are expected to serve as notification that something happened and it is up to the addon to go back to the source of truth (Governor API) and reconcile state with the external system.

Event structure should be well defined and should remain backwards compatible. Addons are expected to leverage constants and types exported by the Governor events package and the versioned API client if they are written in Go.

Events are emitted on NATS subjects. As implemented, the system provides "at most once" delivery guarantees, however infrastructure configuration could be levereged with no changes to the Governor API if addons required better guarrantees. At the time of writting, any addon that needs to ensure state is consistent over time implements an internal reconciler based on a schedule and does not implement any addon managed internal state or storage.

It should be possible for addons to be written by teams outside of the one managing the Governor ecosystem and simply subscribe to the event stream from the Governor API. In the future, it could be valuable to allow addons to publish events as well. This should be added as part of the ecosystem events definitions.

## Governor UI

The Governor UI is an SPA that uses the Governor API for it's backend.

## Auditing

Audit logs are a primary concern of the Governor ecosysystem. All changes are emitted to the audit log from the Governor API and all Governor events carry the `AuditID` with them. This should be propogated and used to emit audit events in addons.
