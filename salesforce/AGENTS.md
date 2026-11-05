# Agent instructions: Salesforce module

These instructions apply to any coding agent (Devin, Claude Code, Cursor, Copilot)
that changes files under `salesforce/`.

## Follow the standards while writing code

Read these before writing Apex, triggers, LWC, or metadata:

1. `salesforce/standards/salesforce-platform.md`: governor limits, bulkification,
   selective SOQL, security, and tests
2. `salesforce/standards/acme-energy-standards.md`: naming, ApexDoc, design, and 90% coverage
3. `salesforce/standards/learned/*.md`: architect-approved rules (these override 1 and 2)

## Review loop before every commit

Run CodeRabbit locally and fix what it finds **before** you commit or push. Repeat
until it reports no findings:

```bash
coderabbit review --agent --uncommitted --include-untracked --dir salesforce --gate major
```

`--gate major` exits 2 while any major or critical finding remains, so an agent
can loop on the exit code. Before opening the PR, run one final pass over the
whole branch:

```bash
coderabbit review --agent --committed --base main --dir salesforce --gate major
```

CodeRabbit uses the same `.coderabbit.yaml`, the standards above, and the
org-registered ApexGuru MCP tools as the PR review. A clean local loop should mean
a near-empty PR review.

## Rules of engagement

- Never weaken a test, an assertion, or a coverage target to make a finding go
  away.
- Do not edit `salesforce/standards/learned/`. It changes only through the
  architect approval workflow.
- If a finding is a false positive, say so in the PR description instead of
  suppressing it.
