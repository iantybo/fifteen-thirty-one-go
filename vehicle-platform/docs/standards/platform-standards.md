# Northstar vehicle platform standards

These standards apply to every service in `vehicle-platform/`, whoever or whatever wrote the code
(a human, Claude Code, Codex, or Gemini). Reviewers, human and CodeRabbit, enforce them on every PR.

## Rollout

- **Ship dark, enable per OEM.** Every new command or behaviour change that reaches a vehicle is gated
  by `northstar.flags.<OEM>` (`OemFeatureFlags`). Code paths must call `OemFeatureFlags.isEnabled`
  before queueing. The flag stays off until that OEM passes certification, which happens in a later
  monthly release.
- **Owners must not notice a deploy.** No change may alter behaviour for vehicles of an OEM whose flag
  is off, and no API or payload change may break a client or head unit already in the field.
- Releases are monthly. Nothing merges straight to production; `main` is cut to a release branch.

## Vehicle command safety

- **Ownership first.** Every endpoint that reads or commands a vehicle calls
  `VehicleOwnershipService.requireOwned(jwt.getSubject(), vin)` before any other work. A VIN alone is
  never authorization. An unknown vehicle and someone else's vehicle return the same 404.
- **Exactly once.** SQS and MQTT QoS 1 are at-least-once. Every command path goes through
  `CommandLedger` (`recordAccepted` on submit, `claimForDispatch` on delivery), so a redelivered message
  never unlocks or starts a car twice.
- **Bounded waits.** Every call to a vehicle, broker, SMS gateway, or OEM cloud passes the configured
  `northstar.vehicle.command-timeout`. Never use `join()` or an untimed `get()` on vehicle I/O, because
  one sleeping TCU would pin a listener thread forever.
- All commands flow through `RemoteCommandService` and `CommandQueueListener`. Feature-specific side
  queues or listeners that skip them are not allowed.

## Data handling

- VINs are personal data. Log `VinMasker.mask(vin)`, never the raw VIN. Never log email, name, phone,
  or location.
- The vehicle shadow table holds every vehicle on the road. Request paths use `GetItem` or `Query`
  (`VehicleShadowRepository`); a DynamoDB `Scan` is never allowed outside offline jobs.
- Read vehicles through `VehicleShadowRepository`; never hand-map DynamoDB items elsewhere.

## Tests

- Every OEM-facing change adds or updates a Cucumber scenario in `src/test/resources/features/` for
  **each OEM and head-unit generation** it affects, including the ones where it must be rejected.
- Unit-test unit conversions and boundaries, including US (Fahrenheit) input.

## Review signal

Human review time is the scarcest resource on this team. Automated review comments are limited to
correctness, safety, security, compatibility, and the standards above. Style, naming, formatting,
Javadoc wording, and refactoring preferences are not review comments.
