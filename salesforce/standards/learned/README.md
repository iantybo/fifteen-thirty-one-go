# Architect-approved review rules

Each file here is a rule that a human architect caught in review and that CodeRabbit
did not. `.github/workflows/architect-feedback.yml` captures it from a `/standard`
review comment and opens a `standards-update` PR. An architect approves it via
CODEOWNERS before it lands.

Once merged, CodeRabbit loads every file here as a code guideline (see
`knowledge_base.code_guidelines` in `.coderabbit.yaml`). Developers get it on
their next `git pull`, so the local `coderabbit` CLI enforces it too.

Only rules merged through that approval flow belong here.
