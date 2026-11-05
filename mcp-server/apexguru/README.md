# apexguru MCP server (simulated)

A **simulated** ApexGuru MCP server for the CodeRabbit Salesforce proof of value.
It stands in for the
[`dx-apexguru-scan`](https://github.com/forcedotcom/sf-skills/tree/main/skills/dx-apexguru-scan)
skill. It gives CodeRabbit "dynamic" analysis: it projects every Apex change
against UAT and production data volumes and runtime telemetry, so code that
passes in a 14-user dev sandbox is flagged before it fails in a 412,000-user UAT.

> All org data is **seeded** in [`seed/`](./seed). This server does not connect
> to Salesforce. Every response carries a "Simulated ApexGuru" banner.

## Why an MCP server and not a skill

The real skill drives the ApexGuru SFAP Scan API using an `sf` CLI org login.
Handing that credential to a review bot, or to every developer's agent, does not
scale and does not pass security review. Instead:

1. An admin runs **one** MCP server inside the company boundary. That server
   holds the Salesforce credentials.
2. The admin registers it **once** in CodeRabbit.
3. Every PR review **and** every local `coderabbit` CLI run can then call the
   tools. No developer installs or configures anything.

Swapping this simulated server for a live one only changes the tool
implementations. `.coderabbit.yaml` and the developer workflow stay the same.

## Tools

| Tool | Purpose |
|------|---------|
| `apexguru_scan` | Static detection of the ApexGuru catalog: `SOQL_IN_LOOP` (direct and one-hop), `DML_IN_LOOP` (direct and one-hop), `GGD`, `SOQL_NO_WHERE_OR_LIMIT`, and `SOQL_UNUSED_FIELDS`. Each finding is enriched with projected impact at `uat`/`prod` volumes ("Production insights"); `dev` is "Static only". Output mirrors the skill's `--present` report. |
| `explain_soql` | Simulated query plan for one SOQL statement across dev, uat, and prod: object rows, Index vs TableScan, estimated rows, and PASS/FAIL against the 50,000-row limit and the non-selective trigger-query threshold. |
| `get_object_volumes` | Seeded sObject row counts per org, plus indexed fields. |
| `get_governor_limits` | Apex per-transaction limits. |
| `get_runtime_hotspots` | Seeded debug-log telemetry per class: CPU, heap, p95 SOQL and DML against limits, and exceptions. |

Isolated `@IsTest` classes, meaning those without `SeeAllData=true`, are not
projected against org volumes, because their data never reaches them.

## Run locally and expose via ngrok

```bash
export APEXGURU_MCP_API_KEY="$(openssl rand -hex 24)"
./run.sh            # builds, serves on :8766 (PORT to override), then runs `ngrok http 8766`
```

`/mcp` requires `Authorization: Bearer <key>` or `X-API-Key: <key>`.
`/healthz` is open. Request bodies are capped at 6 MiB, and a single scan is
capped at 100 files / 2 MiB.

## Wire into CodeRabbit (one time, admin)

CodeRabbit → **Integrations → MCP Servers → Add**:

- URL: `https://<ngrok-id>.ngrok-free.app/mcp`
- Header: `Authorization: Bearer <APEXGURU_MCP_API_KEY>`

`.coderabbit.yaml` sets `knowledge_base.mcp.usage: enabled`. The
"ApexGuru Dynamic Analysis" pre-merge check instructs the reviewer to call
`apexguru_scan` and `explain_soql` for every changed Apex file.

## Test

```bash
go test ./...
```

`TestBaselineSalesforceModuleIsClean` scans `../../salesforce/force-app` at prod
volumes and fails if the committed baseline module has any finding.
