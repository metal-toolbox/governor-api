# Extend Governor APIs

## VIPs

Very interesting problems.

In some cases, an extension or multiple extensions might need to update the
resources upon reconciliation. This could lead to potential issues like the
extensions reacting to its own updates.

### Extensions update loop

```mermaid
sequenceDiagram
  actor u as User
  participant g as Governor API
  participant m as Message Bus
  participant e as Extension


  u->>g: create /things
  activate g
  g->>m: publish event: a thing was created
  activate m
  g->>u: response
  deactivate g

  m->>e: receive event: <br/>a thing was created
  deactivate m
  activate e
  e->>g: fetch thing with ID from event
  g->>e: 
  e->>e: reconciliation
  e->>g: update /things/:id

  activate g
  g->>m: publish event: a thing was updated
  activate m
  g->>e: response
  deactivate g

  deactivate e

  note over g,e: here the extension is reacting to its own update

  m->>e: receive event: <br/>a thing was updated
  deactivate m
  activate e
  e->>g: fetch thing with ID from event
  g->>e: 
  e->>e: reconciliation
  e->>g: update /things/:id

  activate g
  g->>m: publish event: a thing was updated
  activate m
  g->>e: response
  deactivate g

  deactivate e

  note over g,e: so on and so forth an infinite loop is formed

  m->>e: receive event: <br/>a thing was updated
  deactivate m

  activate e
  note over e: .<br/>.<br/>.
  deactivate e
```

The above scenario can be mitigated by allowing the extensions to keep a
local copy (cache) of the resource and only react to changes when the resources
in Governor is actually changed from their own cache.

However, local cache will not solve the issue where there are multiple extensions
reacting to updates on the same resource. More precisely, the extensions will
likely react to the same update $N$ times per extension,
where $N$ is the number of extensions that react to same resource.
(It will eventually stop because eventually the updates will not change any
values in the resource, **unless** the extension is updating the resource
with a unique value that changes every time, e.g. timestamps).

### Annotations

To mitigate this problem, **Annotations** can be introduced
to the extension resources, and having the extension to mark a resource
as `processed`, then ignore any subsequent updates on resources with this
annotation.

### Multiple Extension Race Conditions

```mermaid
sequenceDiagram
  actor u as User
  participant g as Governor API
  participant m as Message Bus
  participant e1 as Extension 1
  participant e2 as Extension 2


  u->>g: create /account
  activate g
  g->>m: publish event: an account was created
  activate m
  g->>u: response
  deactivate g

  m->>e1: receive event: <br/>an account was created
  activate e1

  m->>e2: receive event: <br/>an account was created
  deactivate m

  activate e2

  e1->>g: fetch resource with ID from event
  g->>e1: 

  e2->>g: fetch resource with ID from event
  g->>e2: 

  note over g, e2: resp: { "name": "Alice", balance: 0 }

  e1->>e1: deposite $100
  e2->>e2: deposite $50

  e1->>g: update /accounts/:id
  note over g, e1: payload: { "balance": 100 }
  g->>e1: 
  deactivate e1

  e2->>g: update /accounts/:id
  note over g, e2: payload: { "balance": 50 }
  g->>e2: 
  deactivate e2


  note over g, e2: here Extension 1's update to the resource is discarded, <br/>and Alice's account balance ended up $100 less than it was supposed to be
```

One of the solutions is to avoid having multiple extensions react on the same
extension resource, in doing so, not only will it increase the complexity of the system,
but also introduce a dependency from one extension to another. Nonetheless,
this is still a potential issue to be addressed.

### Resource Versioning

To mitigate this problem, **Resource versioning** can be introduced
to the extension resources. The governor API can generate a new UUID as the
resource version each time the resource is created or updated.
To update a resource, the extension must provide the resource version along
with the patch to the governor API, the the governor API will reject any
updates with resource version mismatches.

### Complete Solution Examples

Here are two examples showing how resource versioning and annotations will address the above
issues.

```mermaid
sequenceDiagram
  actor u as User
  participant g as Governor API
  participant m as Message Bus
  participant e1 as Extension 1
  participant e2 as Extension 2

  note over u, e2: create account example: prevent race condition

  u->>g: create /account
  activate g
  g->>m: publish event: an account was created
  activate m
  g->>u: response
  deactivate g

  m->>e1: receive event: <br/>an account was created
  activate e1

  m->>e2: receive event: <br/>an account was created
  deactivate m

  activate e2

  e1->>g: fetch resource with ID from event
  g->>e1: 

  e2->>g: fetch resource with ID from event
  g->>e2: 

  note over g, e2: resp: { "name": "Alice", balance: 0, "resource_version": "1" }

  e1->>e1: deposite $100
  e2->>e2: deposite $50

  e1->>g: update /accounts/:id
  activate g
  note over g, e1: payload: <br/>{ "balance": 100, "resource_version": "1", "annotations": { "proccessed.extension.governor/ext_1": true }}

  g->>g: update resource version<br/>to "2"
  g->>m: publish event: an account was updated 
  activate m

  g->>e1: 🚀 200 OK
  deactivate e1
  deactivate g

  e2->>g: update /accounts/:id
  activate g
  note over g, e2: payload: <br/>{ "balance": 50, "resource_version": "1", <br/>"annotations": { "proccessed.extension.governor/ext_2": true }}
  g->>e2: ❌ 400 Bad Request: resource version mismatch
  deactivate g

  loop retry until success or backoff
    e2->>g: fetch resource with ID from event
    g->>e2: 

    note over g, e2: resp: <br/>{ "balance": 50, "resource_version": "1", <br/>"annotations": { "proccessed.extension.governor/ext_2": true }}

    e2->>e2: deposite $50
    e2->>g: update /accounts/:id
    activate g
    note over g, e2: payload: <br/>{ "balance": 150, "resource_version": "2",<br/>"annotations": { "proccessed.extension.governor/ext_1": true,<br/>"proccessed.extension.governor/ext_2": true }}
    g->>g: update resource version<br/>to "3"
    g->>m: publish event: an account was updated 
    g->>e2: 🚀 200 OK
    deactivate g
  end

  deactivate e2

  note over u, e2: end of create account example

  note over u, e2: begin of prevent update loop example

  m->>e1: receive event: <br/>an account was updated
  activate e1

  m->>e2: receive event: <br/>an account was updated
  deactivate m

  activate e2

  e1->>g: fetch resource with ID from event
  g->>e1: 

  e2->>g: fetch resource with ID from event
  g->>e2: 

  note over g, e2: resp: <br/>{ "balance": 150, "resource_version": "2",<br/>"annotations": { "proccessed.extension.governor/ext_1": true,<br/>"proccessed.extension.governor/ext_2": true }}

  e1->>e1: verify annotation <br/>"proccessed.extension.governor/ext_1" already exists
  e2->>e2: verify annotation <br/>"proccessed.extension.governor/ext_2" already exists

  e1->>e1: discard event
  deactivate e1

  e2->>e2: discard event
  deactivate e2

  note over u, e2: end of prevent update loop example
```
