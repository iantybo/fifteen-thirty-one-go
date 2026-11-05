# Acme Energy: cribbage signups → Salesforce Leads

An sfdx module that syncs the cribbage app's invite requests
(`backend/internal/handlers/signups.go`) into Salesforce as Leads, enrolls them
in a waitlist Campaign, and gives the CRM team a review queue in Lightning.

It is also the reference project for the **CodeRabbit Salesforce review harness**.

## Components

| Component | Purpose |
|-----------|---------|
| `ACME_SignupRestResource` | `POST /services/apexrest/acme/signups`, called by the cribbage backend |
| `ACME_SignupLeadService` | Validates requests and upserts Leads on `Signup_External_Id__c` (idempotent retries) |
| `ACME_LeadTrigger` → `ACME_LeadTriggerHandler` | The one Lead trigger. It enrolls app Leads in the waitlist Campaign. |
| `ACME_SignupQueueController` + `acmeSignupQueue` LWC | Review queue of recent app signups |
| `ACME_Signup_Integration` permission set | Least-privilege access for the integration user |

## How changes are reviewed

```text
 developer / agent (Devin, Claude Code, Cursor)
        │  writes code following salesforce/standards/** and salesforce/AGENTS.md
        ▼
 coderabbit review --agent --gate major     ◄── loop locally until clean
        │        ├─ platform standards       (salesforce/standards/salesforce-platform.md)
        │        ├─ Acme standards           (salesforce/standards/acme-energy-standards.md)
        │        ├─ learned rules            (salesforce/standards/learned/*.md)
        │        └─ ApexGuru MCP             (mcp-server/apexguru: UAT/prod volumes + telemetry)
        ▼
 git push → GitHub PR → CodeRabbit runs the same review plus 5 blocking pre-merge checks
        ▼
 architect reviews only what automation cannot judge
        │  "/standard <rule>" on a finding CodeRabbit missed
        ▼
 architect-feedback workflow → standards-update PR → architect approves (CODEOWNERS)
        ▼
 merged into salesforce/standards/learned/ → enforced on the next PR and the next local run
```

- **Configured once.** An admin registers the ApexGuru MCP server in CodeRabbit.
  Developers install nothing, and no Salesforce credentials reach CodeRabbit or
  developer machines.
- **Same rules in both places.** The local CLI and the PR review read the same
  `.coderabbit.yaml`, standards, and MCP tools. The local loop shakes out
  findings before push, so the PR review stays light.
- **Dynamic analysis.** Static analyzers see the code. The MCP tools also see
  the org: a query that returns 14 rows in dev and 412,000 in UAT is a failure
  before deployment, not after.

## Using with Gearset

Gearset pipelines still promote through GitHub pull requests. CodeRabbit
reviews the PRs Gearset opens with the same configuration, and Gearset's own
static code analysis complements it. Nothing about this harness depends on the
deployment tool.

## Local development

```bash
sf org create scratch -f config/project-scratch-def.json -a acme-signups
sf project deploy start -o acme-signups
sf apex run test -o acme-signups --code-coverage --result-format human
```
