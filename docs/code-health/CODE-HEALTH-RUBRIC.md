# Code Health Rubric

CodeRabbit uses this rubric to score Kotlin, Swift, C/C++, and PowerShell files
on a 1.0-10.0 scale. Every file starts at 10.0, and each finding below takes
points off. The lowest possible score is 1.0.

| Score      | Rating    | Meaning                                       |
| ---------- | --------- | --------------------------------------------- |
| 8.0 - 10.0 | Healthy   | Easy to change, with low defect risk          |
| 4.0 - 7.9  | Warning   | Changes here are slower and riskier           |
| 1.0 - 3.9  | Unhealthy | Fix this file before adding new features      |

## General deductions (all languages)

| Finding                                     | Deduction  |
| ------------------------------------------- | ---------- |
| Function with cyclomatic complexity > 10    | -1.0 each  |
| Function longer than 60 lines               | -0.5 each  |
| Nesting deeper than 3 levels                | -1.0 each  |
| Duplicated logic block (6+ lines)           | -0.5 each  |
| Function with more than 5 parameters        | -0.5 each  |
| Swallowed error / empty catch block         | -1.5 each  |
| New public code without tests               | -1.0 file  |

## Language-specific deductions

### Kotlin

| Finding                                              | Deduction |
| ---------------------------------------------------- | --------- |
| `!!` on parsed or external input                     | -1.0 each |
| Mutable shared state in an `object` / companion      | -1.5 each |
| Blocking call (`Thread.sleep`, I/O) in a coroutine   | -1.5 each |

### Swift

| Finding                                              | Deduction |
| ---------------------------------------------------- | --------- |
| Force unwrap, `try!`, or `as!` on runtime data       | -1.0 each |
| Retain cycle from a strong `self` capture            | -1.5 each |
| Unsynchronized shared state across queues or tasks   | -2.0 each |

### C/C++

| Finding                                              | Deduction |
| ---------------------------------------------------- | --------- |
| Unchecked buffer write (strcpy, sprintf, indexing)   | -2.0 each |
| Memory leak, double free, or use-after-free          | -2.0 each |
| Uninitialized read                                   | -1.5 each |
| Signed/unsigned comparison or integer overflow bug   | -1.0 each |

### PowerShell

| Finding                                                | Deduction |
| ------------------------------------------------------ | --------- |
| `Invoke-Expression` on external input                  | -2.0 each |
| Plaintext secret                                       | -2.0 each |
| Unguarded `Remove-Item -Recurse -Force`                | -2.0 each |
| `$ErrorActionPreference = 'SilentlyContinue'`          | -1.0 each |
| `$x -eq $null` (should be `$null -eq $x`)              | -0.5 each |

## Hotspots and knowledge risk

- **Hotspot score** = commits in the last 90 days x (10 - Code Health score).
- **Knowledge island**: one author wrote more than 75% of the file.
- **Fix First**: a hotspot that is also a knowledge island, or that has no tests.

## Tracking AI-assisted code over time

The `AI-Generated Code Health Tracking` pre-merge check ends with one line in
this format:

```text
CODE_HEALTH_TREND ai_files=<n> ai_avg=<score> ai_delta=<delta> human_files=<n> human_avg=<score> human_delta=<delta>
```

To chart the trend across merged PRs, pull those lines from the CodeRabbit
comments:

```bash
gh pr list --state merged --limit 200 --json number --jq '.[].number' |
  while read -r pr; do
    gh pr view "$pr" --comments --json comments \
      --jq '.comments[].body' | grep -o 'CODE_HEALTH_TREND.*' | sed "s/^/#$pr /"
  done
```
