# vehicle-platform

Northstar's new platform services: the cloud-side proxy for every connected vehicle on the road. Owner
apps and partners send remote commands (lock, unlock, remote start, horn/lights, and climate) here. The
platform checks ownership and rollout flags, records each command for exactly-once delivery, and
dispatches it through a per-OEM adapter that handles that manufacturer's transport, payload version,
and wake-up quirks.

```text
owner app ─► RemoteCommandController ─► RemoteCommandService ─► SQS ─► CommandQueueListener ─► OemAdapter ─► vehicle
              (ownership)                (capability, flags,              (claim in ledger)        (Avalon / Brightline / Cascade)
                                          idempotency ledger)
```

- **Stack:** Java 21, Spring Boot 3, Gradle, AWS (DynamoDB, SQS), MQTT 5 (HiveMQ client), JUnit 5,
  Mockito, and Cucumber.
- **Standards:** [`docs/standards/`](docs/standards/)
- **Agents:** [`AGENTS.md`](AGENTS.md)

```bash
gradle test   # Gradle 8.10+; no wrapper is committed in this POV
```
