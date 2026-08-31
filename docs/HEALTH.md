# Health & readiness endpoints

Two unauthenticated endpoints outside the `/api` group, intended for orchestrators,
load balancers, and uptime checks.

They are deliberately split: **liveness** answers "is this process alive?" and
**readiness** answers "should this process receive traffic?". Conflating them makes a
brief database outage look like a crashed process, so an orchestrator restarts an
instance that would have recovered on its own.

## `GET /healthz` — liveness

Checks nothing but the process itself, and always returns `200` while the server can
answer HTTP. Use it for restart decisions.

```json
{"ok": true}
```

## `GET /readyz` — readiness

Checks each dependency and returns `200` when all pass, `503` when any fails. Use it
for load-balancer membership: a failing instance drains without being restarted.

| Check | Meaning |
| --- | --- |
| `database` | `PingContext` succeeds within 2s |
| `websocket_hub` | A hub is configured and has not been stopped |

```json
{
  "ok": true,
  "version": "(devel)",
  "uptime_seconds": 142,
  "checks": {
    "database": "ok",
    "websocket_hub": "ok"
  }
}
```

A failing check reports a short reason, and every check runs regardless of whether an
earlier one failed — so one probe shows everything that is wrong, not just the first
problem:

```json
{
  "ok": false,
  "version": "(devel)",
  "uptime_seconds": 142,
  "checks": {
    "database": "unavailable: ping failed",
    "websocket_hub": "ok"
  }
}
```

`version` comes from the embedded build metadata: `(devel)` under `go run`, the module
version for a released build, and `unknown` when no build info is available.

## Notes

- Because both endpoints are unauthenticated, failure reasons are fixed strings. The
  underlying driver error is not echoed, so connection details are not exposed; check
  the server logs for specifics.
- The database ping is bounded at 2s, comfortably under the server's 15s write timeout,
  so a wedged database fails the probe instead of hanging it.
- `uptime_seconds` is measured from process start, which makes an unexpected restart
  visible as a reset without needing to correlate logs.
