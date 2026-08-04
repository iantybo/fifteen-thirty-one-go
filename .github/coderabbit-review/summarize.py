#!/usr/bin/env python3
"""Render a review JSON payload into GHA outputs and a job summary.

The `--agent` JSON schema is not publicly documented, so this is deliberately
defensive: when it cannot recognize the shape it reports an unknown count and
dumps the raw JSON into the summary rather than failing the step or silently
claiming zero findings.
"""

import json
import os
import re
import sys

SEVERITY_KEYS = ("severity", "level", "priority")
FILE_KEYS = ("fileName", "file", "path", "file_path", "filename")
LINE_KEYS = ("line", "line_number", "start_line", "lineNumber")
TEXT_KEYS = (
    "comment",
    "summary",
    "message",
    "title",
    "description",
    "body",
    "codegenInstructions",
)
LIST_KEYS = ("findings", "comments", "issues", "review_comments", "results")

# Non-finding event types in the `--agent` JSON Lines stream.
PROGRESS_TYPES = ("status", "heartbeat", "review_context", "progress")


def redact(text):
    return re.sub(r"\bcr-[A-Za-z0-9_\-]{8,}", "***redacted***", text or "")


def find_findings(review):
    """Locate the list of findings, or None if the shape is unrecognized.

    `review --agent` emits JSON Lines: progress events (`status`, `heartbeat`,
    `review_context`) interleaved with `finding` objects. When handed that event
    list, select only the findings -- counting every event would report
    heartbeats as findings.
    """
    if isinstance(review, list):
        typed = [e for e in review if isinstance(e, dict) and "type" in e]
        if typed:
            findings = [e for e in typed if e.get("type") == "finding"]
            # An all-progress stream is a real, empty review -- not an
            # unrecognized shape.
            if findings or any(e.get("type") in PROGRESS_TYPES for e in typed):
                return findings
        return review
    if not isinstance(review, dict):
        return None
    for key in LIST_KEYS:
        value = review.get(key)
        if isinstance(value, list):
            return value
    for value in review.values():
        if isinstance(value, dict):
            nested = find_findings(value)
            if nested is not None:
                return nested
    return None


# Every `codegenInstructions` opens with the same agent preamble; it carries no
# information about the finding and would otherwise fill the summary table.
BOILERPLATE = re.compile(
    r"^\s*Verify each finding against current code\..*?validate\.\s*", re.S
)


def describe(item):
    """Human-readable text for a finding, with agent boilerplate stripped."""
    text = str(pick(item, TEXT_KEYS))
    return BOILERPLATE.sub("", text).strip() or text


def pick(item, keys, default=""):
    if not isinstance(item, dict):
        return default
    for key in keys:
        value = item.get(key)
        if value not in (None, "", []):
            return value
    return default


def cell(value, limit=160):
    text = str(value).replace("|", "\\|").replace("\n", " ").strip()
    return (text[: limit - 1] + "…") if len(text) > limit else text or "—"


def write_output(name, value):
    path = os.environ.get("GITHUB_OUTPUT")
    if not path:
        print("%s=%s" % (name, value))
        return
    with open(path, "a") as handle:
        if "\n" in str(value):
            # Delimiter form: required for multiline values like the raw JSON.
            delim = "ghadelim_%s" % name
            handle.write("%s<<%s\n%s\n%s\n" % (name, delim, value, delim))
        else:
            handle.write("%s=%s\n" % (name, value))


def write_summary(lines):
    path = os.environ.get("GITHUB_STEP_SUMMARY")
    text = "\n".join(lines) + "\n"
    if not path:
        sys.stdout.write(text)
        return
    with open(path, "a") as handle:
        handle.write(text)


def main():
    review_file = sys.argv[1] if len(sys.argv) > 1 else "review.json"
    try:
        with open(review_file) as handle:
            payload = json.load(handle)
    except (OSError, ValueError) as exc:
        write_output("status", "error")
        write_output("finding-count", "unknown")
        write_summary(["## CodeRabbit review", "", "Could not read review output: `%s`" % exc])
        return 1

    review = payload.get("review", payload) if isinstance(payload, dict) else payload
    # The server already separates findings from progress events; trust that
    # when present and only fall back to sniffing the raw stream.
    if isinstance(payload, dict) and isinstance(payload.get("findings"), list):
        findings = payload["findings"]
    else:
        findings = find_findings(review)
    raw_json = redact(json.dumps(review, indent=2))

    write_output("status", str(payload.get("status", "ok")) if isinstance(payload, dict) else "ok")
    write_output("review-json", raw_json)
    write_output("review-file", os.path.abspath(review_file))

    lines = ["## CodeRabbit review", ""]

    if findings is None:
        # Unrecognized shape: say so plainly instead of reporting a fake zero.
        write_output("finding-count", "unknown")
        lines += [
            "Could not identify a findings list in the `--agent` output; "
            "showing the raw payload.",
            "",
            "<details><summary>Raw review JSON</summary>",
            "",
            "```json",
            raw_json[:60_000],
            "```",
            "",
            "</details>",
        ]
        write_summary(lines)
        return 0

    write_output("finding-count", str(len(findings)))

    if not findings:
        lines += ["No findings. :white_check_mark:"]
        write_summary(lines)
        return 0

    lines += [
        "**%d finding%s**" % (len(findings), "" if len(findings) == 1 else "s"),
        "",
        "| File | Line | Severity | Comment |",
        "| --- | --- | --- | --- |",
    ]
    for item in findings[:200]:
        lines.append(
            "| %s | %s | %s | %s |"
            % (
                cell(pick(item, FILE_KEYS), 80),
                cell(pick(item, LINE_KEYS), 12),
                cell(pick(item, SEVERITY_KEYS), 20),
                cell(redact(describe(item))),
            )
        )
    if len(findings) > 200:
        # Never truncate silently.
        lines += ["", "_Showing first 200 of %d findings._" % len(findings)]

    lines += [
        "",
        "<details><summary>Raw review JSON</summary>",
        "",
        "```json",
        raw_json[:60_000],
        "```",
        "",
        "</details>",
    ]
    write_summary(lines)
    return 0


if __name__ == "__main__":
    sys.exit(main())
