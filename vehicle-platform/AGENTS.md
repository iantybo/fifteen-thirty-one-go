# Agent instructions: vehicle-platform

This file is the shared instruction set for every coding agent on this team: Claude Code, Codex,
Gemini, or anything else. `CLAUDE.md` and `GEMINI.md` point here, so all models work from the
same rules.

Before you write code, read:

1. `docs/standards/platform-standards.md`: rollout flags, ownership, idempotency, timeouts, and data handling
2. `docs/standards/oem-capability-matrix.md`: what each OEM and head-unit generation can do

Non-negotiables:

- New vehicle-facing behaviour ships behind `OemFeatureFlags`, off by default.
- Never change `CommandPayloadV1`. Use `CommandPayloadV2.params`.
- Route commands through `RemoteCommandService` and `CommandQueueListener`; never add a side queue.
- Add Cucumber scenarios for every OEM and generation you touch.
- Mention in the PR description that the change was AI-assisted, and which agent produced it.

Before opening a PR, run the local review and fix what it finds:

```bash
coderabbit review --agent --uncommitted --include-untracked --dir vehicle-platform --gate major
```
