# Config Bus: change notification for gccs (design)

Status: proposal

## Motivation

Spring Cloud Config Server is usually paired with Spring Cloud Bus, which broadcasts "config changed" events so that
clients can refresh without a restart. gccs is a drop-in for the server half, but non-JVM clients have no bus.
This document describes a NATS-based equivalent.

Today, clients treat config as fixed for the lifetime of the process and validate it once, at startup. Introducing
runtime change breaks that assumption, so most of this document is about *safety*, not delivery.

## Goals

* Notify interested clients, promptly, that config they use has changed.
* Let clients validate and apply (or reject) a change without compromising availability.
* Make every outcome visible: which version each instance runs, and why a change was rejected.
* Never make config *serving* less reliable than it is today.

## Non-goals

* Pushing config values over the bus. Events carry identifiers only; clients re-fetch through the normal,
  authenticated gccs API.
* Mandating hot reload. Reload is opt-in per client, and per field.
* Requiring NATS JetStream. All designs here work with core NATS; JetStream may be used where available.

## Architecture

```
 git remote ──poll/webhook──▶ gccs-bus ──publish──▶ NATS ──▶ clients
                                │  ▲                          │
                     resolve    │  └────── status ────────────┘
                    (HTTP GET)  ▼
                              gccs ◀──── fetch (pinned version) ── clients
                                │
                          reads ConfigMap (optional, lastgood)
```

The bus is a **separate deployable** from gccs, with no code dependency on it (it talks to gccs over HTTP only).
Rationale:

* gccs is on the startup path of every client. Keeping NATS credentials, status handling, Kubernetes write access and
  new state out of it means a bus or NATS fault cannot affect config serving.
* A single publisher avoids duplicate events from multiple gccs replicas, and lets the bus wait until *every* replica
  serves a new version before announcing it (replicas pull git independently and can briefly disagree).
* Publish credentials and write permissions are confined to one small component.

If the bus is down, no events are emitted. Clients keep running on their current config. This is the safe failure.

## Change detection

The bus polls the remote's refs (no clone needed) or receives a push webhook (shared-secret verified). On a new
commit it computes the changed paths and maps them to affected applications and profiles using the standard naming
convention (`<app>-<profile>.yml`, `<app>.yml`, `application-<profile>.yml`, `application.yml`).

A change to a shared file (`application*.yml`) affects every application. It is published on a separate, explicitly
enabled subject (see below), not fanned out by default.

## Pre-publish checks

Before announcing a version the bus verifies, via the gccs API:

1. Every gccs replica serves the new commit (resolved through a headless service).
2. Each affected `app/profiles` resolves successfully at that commit (`GET /{app}/{profiles}/{commit}`).

If either fails, nothing is published, and the failure is logged and exposed as a metric. This catches broken YAML,
unresolvable placeholders and precedence errors, but it cannot catch *semantic* errors, because gccs does not know
client schemas. It also cannot catch errors that only arise when several applications are loaded *together* by one
consumer (see below). That is the clients' job, below.

## Consumers and load sets

A client (a *consumer*) typically loads several configuration applications in one request, e.g. its own plus a shared
one plus `application`, and resolves them for an ordered list of active profiles. Consequences:

* A consumer subscribes to the `changed` subject of **every** application in its load set, and matches the event's
  `profiles` against its own active profile list. A change to one file can therefore reach several different consumers.
* Validation may span applications in the load set (e.g. a shared schedule that must reference names declared by the
  consumer's own config). The same change can be valid for one consumer and invalid for another. **Partial rejection
  across consumers is a normal outcome**, not a fleet-wide fault.
* Status and last-known-good are therefore tracked per *consumer*, not per configuration application.
* Falling back to last-known-good pins the **whole load set** to a single commit, so a consumer never mixes an old
  shared file with a new private one.

## Events

Subject layout (prefix configurable):

| Subject | Purpose |
| --- | --- |
| `gccs.config.changed.<app>` | App-specific change |
| `gccs.config.changed._shared` | Shared files changed; opt-in subscription |
| `gccs.config.status.<consumer>` | Client status reports, keyed by the consuming service |

Change event (identifiers only, never values):

```json
{
  "id": "<unique event id>",
  "version": "<commit sha>",
  "previous": "<commit sha>",
  "applications": ["payments"],
  "profiles": ["prod"],
  "paths": ["payments-prod.yml"],
  "timestamp": "2026-01-01T00:00:00Z"
}
```

* `id` uniquely identifies the *event*. A catch-up or re-published event for the same `version` gets a new `id`.
  Clients compare `version` to decide whether anything needs applying, and use `id` to correlate status reports.
* `paths` lists the changed files. It is informational: it helps with debugging and lets a client skip a reload if
  nothing relevant to it changed. Clients must not depend on it being present.

Delivery is at-most-once. Clients must therefore also run a slow periodic check against gccs (compare the applied
version with the current one), which doubles as the backstop for missed events.

## Client contract

This is the part that matters most.

### Apply: candidate, validate, swap

On an event, a client must never mutate live config in place.

1. Fetch the configuration pinned to the event's `version`. (Pinning avoids racing a newer commit.)
2. Build a complete candidate and run the same validation used at startup. Startup validation can have several
   phases (structural checks, then checks needing runtime state); the candidate must pass all of them.
3. For each watched group (below) whose values changed, validate it **in the context of the candidate**, then apply it
   atomically so readers see the old or the new value, never a mix.
4. If a group is invalid, keep its current value and report the rejection. Other groups are unaffected.

### Granularity: watched groups

Reloading a whole configuration object is usually impractical: it is captured by many components, and most of it
(connection settings, registries, anything consumed once at startup) cannot change in a live process. Instead,
clients register **watched groups**: a set of configuration paths that are validated and applied together.

```go
// Illustrative API only.
h := Watch[Schedules](loader, "schedules",
    Validate(func(candidate Schedules, ctx Candidate) error { … }), // may inspect other groups and fixed values
    OnApply(func(old, new Schedules) { … }))                         // rebuild derived state
current := h.Get()                                                  // atomic read
```

* The unit is a *group*, not necessarily a single property. Related values (a schedule and the names it refers to; a
  pair of buffers) must change together, because applying them one at a time exposes transient states that are
  invalid. A single path is just a group of one.
* A client must read through a **handle with an atomic getter**, not through a pointer that the library writes to.
  In-place writes to a field, and especially to a slice or map, race with concurrent readers.
* Paths are given explicitly; a pointer to a field does not identify its key. Lists arrive flattened, so a list is
  watched by prefix.
* Validators receive the candidate values of the other watched groups plus the current values of everything fixed.
* `OnApply` rebuilds derived state; the group's value alone is rarely the whole story.
* Registering a group is the declaration that it is reloadable. Everything else is restart-required.
* Because groups succeed or fail independently, a process can run groups from different commits. Status therefore
  reports versions per group (below), and startup fallback (below) is the mechanism that pins the whole load set.

Further rules:

* **Never fail aggressively.** A rejected or failed reload must not exit the process and must not affect liveness or
  readiness. Degraded config state is reported through logs, metrics and the status subject, not the health probes.
* **Recover from panics** in the reload path.
* **Restart-required changes.** A change to a path that is not part of any watched group is not applied silently: the
  client reports `restart-required`.
* **Loader libraries must return errors, not exit.** A reload path that can call a fatal-logging function (on a bad
  override string, a failed validation, a malformed response) defeats the rule above.
* **Check the response.** Verify the HTTP status and that a version was returned before trusting a body.
* **Re-apply local overrides.** Anything layered over fetched config (environment-variable overrides, injected
  properties) must be layered again on each reload, or it will silently disappear.
* **Beware load-time side effects.** If loading also performs side effects (generating credentials, auto-enabling
  features), the reload path must not repeat them blindly; separate pure loading from side effects.
* **Retain the applied version.** The version returned with a fetch is what the client reports; keep it.
* **Optional post-apply check.** Keep the previous config in memory for a grace window and roll back automatically if
  a user-supplied health check fails. Rollback is in-memory, never an exit.
* **De-duplicate** events: ignore an `id` already seen, and ignore versions already applied or older than the
  current one.
* **Jitter** the re-fetch by a random delay so that a broad change does not stampede gccs.
* **Do not retry** a rejected version in a tight loop. Wait for the next version or the slow periodic check.
* **Never log values.** Rejection reasons name the field path and the rule broken, not the data.
* Reload is opt-in. Clients that hold sensitive state may choose to require a restart instead.

### Status reporting

Clients publish on `gccs.config.status.<consumer>`:

```json
{
  "event": "<id of the change event, if triggered by one>",
  "consumer": "payments",
  "instance": "payments-7f9c",
  "group": "schedules",
  "version": "<commit sha>",
  "state": "applied | rejected | restart-required | rolled-back",
  "reason": "field path and rule only",
  "timestamp": "2026-01-01T00:00:00Z"
}
```

A consumer's overall state is derived from its groups: it is "at" version V only when every watched group is.

Clients should additionally expose the applied version (a gauge metric, and on any existing diagnostic endpoint) so
fleet divergence from the latest commit is alertable.

## The latent-bad-config problem

Rejecting a bad version at runtime keeps running instances healthy, but the bad commit is still in git, so *any new
instance* (scale-up, eviction, crash) will fetch it and fail validation. Rejection therefore has to be loud, and
startup needs the same forgiveness as runtime.

### Startup fallback

1. The client fetches the latest config and validates it.
2. If validation fails, it reports `rejected` and fetches the **last-known-good** version instead.
3. It starts in a degraded state (alert raised, readiness unaffected) rather than crash-looping.
4. If no last-known-good exists, or it also fails, the client fails as it does today.

The client makes this decision, and reports it, rather than gccs silently serving older config: silent substitution
would hide intended changes such as secret rotations.

### Last-known-good

The bus maintains a per-consumer *last-known-good* version (a commit that the consumer's whole load set resolved and
applied under). A version is promoted when:

* the consumer's instances have reported every watched group `applied` (a minimum count, configurable), **and**
* it has run without rollback for a minimum soak time (configurable).

Soak time matters because applications with one or two instances make a count-based quorum a weak signal.
Only positive evidence moves the pointer, and only forward along git ancestry, so lost or late status messages can
delay promotion but cannot make the pointer unsafe.

gccs exposes it by resolving a reserved label (`label=lastgood`), so the client's fallback is one more request to the
service it already uses, with no new startup dependency on the bus. This is the only change proposed for gccs, and
it is optional (gccs reads the state read-only).

### State storage (Kubernetes)

State is stored in a ConfigMap, requiring neither JetStream nor a database:

```yaml
data:
  payments: '{"lastGood":"a1b2c3","candidate":"d4e5f6","candidateSince":"…","lastPublished":"d4e5f6","applied":3,"rejected":0}'
```

* Updates use the ConfigMap's `resourceVersion` for optimistic concurrency; on conflict, re-read and recompute.
* Writes happen on state transitions (candidate set, promotion, rejection threshold crossed), not per status message.
* Write RBAC is scoped to that one named ConfigMap, and held by the bus only. gccs needs read access.
* Losing the ConfigMap degrades conservatively: no last-known-good is available, and behaviour reverts to today's.
* On restart, the bus compares `lastPublished` with the current HEAD and publishes a catch-up event if needed.

## Circuit breaker

If rejections for a version cross a configurable threshold *for a given consumer*, the bus stops publishing further
events that affect that consumer until a new commit arrives, raises an alert, and may publish an advisory on the
status subject. Rejections are counted per consumer, because a change one consumer rejects and another applies is
expected (see "Consumers and load sets"). The bus cannot recall events already sent; rollback is a git revert, which
produces a new version and a new event.

## Rollout controls

* **Off by default**, with a runtime switch to suspend publishing without a redeploy.
* **Shared-file changes** are published only if explicitly enabled.
* **Environment ordering** (optional): hold promotion for one environment until the same content reports `applied`
  in an earlier one.
* **Authorization:** only the bus's NATS identity may publish on `gccs.config.>`; clients are subscribe-only on
  `changed` subjects and publish-only on their own `status` subject.

## Observability

* Bus: versions detected, published, withheld (with reason), breaker state, lastGood per consumer.
* Clients: applied version gauge, reload outcome counters by state, time to apply.
* Alerts: rejection on any instance; applied-version divergence beyond a window; fallback in use; breaker open.

## Differences from Spring Cloud Bus

This design borrows the general shape of Spring Cloud Bus (events broadcast on a message bus, changed paths mapped to
applications, optional acknowledgements) but departs from it deliberately in several places. Readers familiar with
Spring should note:

| Area | Spring Cloud Bus / Config Client | This design |
| --- | --- | --- |
| Event payload | Some event types can carry property key/values | Identifiers only; values are always re-fetched through the authenticated gccs API |
| Fetch on change | Client fetches whatever the label currently resolves to | Client fetches pinned to the event's commit `version`, so a newer commit cannot race the notification |
| Applying a change | Environment is refreshed, then bound properties and refresh-scoped beans are rebuilt | Complete candidate is built and validated first, then swapped atomically; failure leaves the live config untouched |
| Failure behaviour | A bad value can surface during rebinding, after the environment has changed | Rejection never affects process health; last-known-good keeps serving |
| Acknowledgements | Optional "received" event | Status reports the *outcome* (`applied`, `rejected`, `restart-required`, `rolled-back`) |
| Startup | Fail-fast and retry options | Falls back to last-known-good when the latest config fails validation |
| Targeting | Destination patterns (`app:profile:instance`) | Subject per application; shared-file changes on a separate opt-in subject |

The server API is unchanged: pinning by `label=<commit>` is part of the standard Config Server contract that gccs
already implements, so existing clients remain compatible.

## Implementation phases

1. Bus skeleton: ref polling, changed-path mapping, no-op publisher, dry-run logging of what *would* be published.
2. Pre-publish checks against gccs, including the all-replicas-converged check.
3. NATS publisher and status subscriber.
4. ConfigMap state store, last-known-good promotion with soak time, circuit breaker.
5. gccs: `label=lastgood` support.
6. Client documentation and a reference client library.

Phases 1 and 2 are useful on their own: dry-run mode lets the logic be observed against real commits before anything
reaches NATS. Phase 5 should land before the bus is enabled anywhere that matters, since it provides the startup
safeguard.

## Open questions

* Exact reserved label name and response shape for the last-known-good lookup.
* Promotion thresholds (count and soak time) and their defaults for low-instance applications.
* Whether the status subject should carry a signed or authenticated identity beyond NATS account permissions.
* Whether to offer an optional JetStream mode (durable status, replay) where available.
