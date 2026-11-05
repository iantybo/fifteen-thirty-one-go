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
`/healthz` is open. `/mcp` also checks the `Host` header against an
allowlist: loopback, plus `APEXGURU_ALLOWED_HOSTS` (comma-separated; a leading
`.` matches subdomains; the default is the ngrok domains). This replaces the
SDK's localhost-only rebinding guard, which would reject ngrok traffic. Request bodies are capped at 6 MiB, and a single scan is
capped at 100 files / 2 MiB.

## Wire into CodeRabbit (one time, admin)

CodeRabbit → **Integrations → MCP Servers → Add**:

- URL: `https://<ngrok-id>.ngrok-free.app/mcp`
- Header: `Authorization: Bearer <APEXGURU_MCP_API_KEY>`
- Name / label: `apexguru`. `.coderabbit.yaml` refers to the server by this name.
- Description / usage prompt:

```text
apexguru is the Salesforce ApexGuru performance-analysis server for this organization. It knows the
data volumes, indexes, and runtime telemetry of the dev, uat, and prod orgs, which the source code
cannot show. Code that passes in a 14-user dev sandbox can fail in a 412,000-user UAT org; this server
is how you find that before merge. Its org data is simulated for this proof of value.

Use it whenever a review touches Salesforce Apex (.cls, .trigger) or an LWC that calls Apex:
- apexguru_scan(files[{path, content}], environment): run on the full content of every changed Apex
  file, with environment "uat" and again with "prod". It returns governor-limit antipatterns (SOQL/DML
  in loops, including one hop through called methods; non-selective queries; unbounded queries;
  getGlobalDescribe; unused fields) with severity, file:line, and production impact.
- explain_soql(query, environment, context): run on every added or modified SOQL statement, even when
  it has a WHERE clause. context is "trigger" for trigger and *TriggerHandler code, "sync" otherwise,
  and "batch" for Database.Batchable start methods. FAIL in uat or prod is a merge blocker.
- get_runtime_hotspots(class_name, environment): run on every changed class that already exists,
  to see how much SOQL, DML, and CPU headroom its transaction has left before adding work to it.
- get_object_volumes(environment, object) and get_governor_limits(): reference data for explaining
  findings.

When reporting a finding from these tools, quote the org, row counts, and limit message, and state
whether dev passes. Treat Critical (1) and High (2) results as blocking. Never report these findings
as theoretical: they are projected against real org volumes. If a tool call fails, say the analysis
could not run instead of assuming the code is safe.
```

`.coderabbit.yaml` sets `knowledge_base.mcp.usage: enabled`. The
"ApexGuru Dynamic Analysis" pre-merge check and the `salesforce/**/*.{cls,trigger}`
and LWC path instructions tell the reviewer which tools to call for each file type.

## Test

```bash
go test ./...
```

`TestBaselineSalesforceModuleIsClean` scans `../../salesforce/force-app` at prod
volumes and fails if the committed baseline module has any finding.
