import { defineConfig } from "@coderabbitai/config"

/**
 * CodeRabbit configuration-as-code.
 *
 * Docs: https://docs.coderabbit.ai/configuration/typescript-configuration
 *
 * Note: a committed `.coderabbit.yaml`/`.coderabbit.yml` always takes
 * precedence over this file, so the repo must not carry one.
 *
 * Verify the resolved config on any PR with: `@coderabbitai configuration`
 */
export default defineConfig({
  knowledge_base: {
    automatic_repository_linking: true,
    code_guidelines: {
      filePatterns: [],
    },
  },

  reviews: {
    profile: "chill",

    enable_prompt_for_ai_agents: true,
    slop_detection: {
      enabled: true,
    },

    pre_merge_checks: {
      override_requested_reviewers_only: false,
      custom_checks: [],
    },

    suggested_reviewers: true,
    auto_assign_reviewers: false,
    suggested_reviewers_instructions: [],

    suggested_labels: true,
    auto_apply_labels: false,
    labeling_instructions: [],
    mutually_exclusive_groups: {},

    assess_linked_issues: true,
    related_issues: true,
    related_prs: true,

    high_level_summary: true,
    high_level_summary_in_walkthrough: true,

    auto_review: {
      enabled: true,
      drafts: true,
      base_branches: [".*"],
    },

    sequence_diagrams: true,
    changed_files_summary: true,
    collapse_walkthrough: true,
    estimate_code_review_effort: false,
    auto_title_placeholder: "[cr]",
    auto_title_instructions: "Follow: https://www.conventionalcommits.org/en/v1.0.0/",
    in_progress_fortune: false,
    review_details: false,
    poem: false,

    commit_status: false,
    fail_commit_status: false,
    review_progress: false,
    review_status: false,

    request_changes_workflow: false,
  },

  inheritance: true,

  chat: {
    art: false,
    auto_reply: true,
  },
})
