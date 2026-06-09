# service-catalog MCP server

An MCP server that publishes the cribbage platform's microservice fleet
catalog. For each of the 30 services in `uservices/` it reports:

- **tier** — production impact tier. **Tier 1 is the highest service impact**
  (a regression risks a customer-facing outage); **tier 4 is the lowest**.
- **argo_rollout** — `yes` if the service deploys via an Argo Rollout
  (progressive delivery), `no` for a plain Kubernetes Deployment.

The data lives in [`services.csv`](./services.csv) (embedded at build time) and
is **fixed/seeded**, not randomized at runtime, so reviewers get a stable
answer on every lookup. It mirrors `uservices/pkg/catalog`.

## Transport & auth

The server speaks MCP over a **streamable HTTP** transport at `/mcp`, so it can
be exposed publicly with **ngrok**.

The `/mcp` endpoint requires a **static API key**, supplied via the
`SERVICE_CATALOG_API_KEY` environment variable. Clients must send it as either:

```
Authorization: Bearer <key>
X-API-Key: <key>
```

`/healthz` is unauthenticated for liveness probes.

## Tools

- `list_services` — returns the catalog as CSV (`name,tier,argo_rollout`).
  Optional `tier` (1–4) argument filters to one tier.
- `get_service` — looks up a single service by `name`.

## Resource

- `catalog://services.csv` — the full catalog as a `text/csv` resource.

## Run locally + expose via ngrok

```bash
SERVICE_CATALOG_API_KEY=your-secret ./run.sh
```

This builds the binary, starts it on `:8765` (override with `PORT`), and runs
`ngrok http 8765`. ngrok prints a public `https://<id>.ngrok-free.app` URL; the
MCP endpoint is that URL plus `/mcp`.

## Wiring into CodeRabbit

Add the public endpoint as an MCP server in CodeRabbit's settings (the
`.coderabbit.yml` `knowledge_base.mcp` block keeps MCP usage enabled). Point it
at `<ngrok-url>/mcp` with the `Authorization: Bearer <key>` header. CodeRabbit
is instructed (see the repo `.coderabbit.yml` path instructions) to call
`get_service` / `list_services` to assess the blast radius of a change by the
owning service's tier.
