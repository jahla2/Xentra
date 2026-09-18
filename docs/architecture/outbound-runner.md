# Outbound Runner Control Channel

The recommended Runner transport is outbound-only:

```
Xentra Control Plane
      |
      | PostgreSQL runner_tasks
      |
      v
Dedicated Runner Control Listener (:8443)
      ^
      | HTTPS + mutual TLS + Runner token
      |
Target Environment
  Xentra Runner
      |
      +-- system.* read tools
      +-- docker.* read tools
      +-- approved restart mutations
```

## Enrollment

An organization owner creates an outbound Runner enrollment for a project/environment. Xentra creates:

- an environment with `connectionType=runner_outbound`;
- a Runner ID;
- a cryptographically random Runner token stored encrypted at rest;
- a one-time UI response containing the Runner ID and token.

The token is only displayed during enrollment. It is not returned by environment listing APIs.

## Transport authentication

Production requires two independent controls:

1. TLS 1.3 mutual authentication on the dedicated Runner-control listener.
2. The per-Runner `Authorization: Runner <token>` credential.

The server certificate is verified by the target Runner. The control plane verifies that the Runner client certificate chains to `XENTRA_RUNNER_CONTROL_CLIENT_CA`. The token then binds the request to the enrolled Runner record.

Local development may explicitly set `XENTRA_RUNNER_CONTROL_INSECURE_DEV=true` on the control plane and `XENTRA_CONTROL_INSECURE_DEV=true` on the Runner. These flags must not be used in production.

## Task queue

Typed tasks are persisted in PostgreSQL. Runners poll for work, and PostgreSQL claims one queued task using `FOR UPDATE SKIP LOCKED`.

States:

- `queued`
- `claimed`
- `completed`

A claimed task older than the lease window is returned to `queued` so another Runner process can recover it after a crash. Result completion is idempotent, allowing the Runner to retry delivery if the network drops after the server commits the result.

The Runner sends discovery information on poll. Xentra updates the environment OS, hostname, capabilities, and last-seen timestamp from authenticated Runner traffic.

## Safety boundary

The outbound channel does not create a generic shell primitive. The control plane continues to dispatch typed tools only.

Read-only investigation tools are automatically available according to discovered capabilities. State-changing tools such as `docker.restart` and `system.service_restart` can only enter the queue after the existing owner approval policy succeeds.

## Migration

Existing `runner` environments continue to use the legacy inbound mTLS HTTP transport. New environments should use `runner_outbound`. Production can set `XENTRA_LEGACY_RUNNER_ENABLED=false` once legacy environments have been migrated.
