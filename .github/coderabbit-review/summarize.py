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
SUGGESTION_KEYS = ("suggestions", "suggestion", "patch", "diff", "fix")

# Order findings worst-first; unknown severities sort last but keep their label.
SEVERITY_ORDER = ("critical", "major", "minor", "trivial", "info")

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
    """Squash a value onto one line for the index table.

    Only used for short fields (file, line, severity) and the at-a-glance
    index; finding bodies are rendered in full by `render_finding`.
    """
    text = str(value).replace("|", "\\|").replace("\n", " ").strip()
    return (text[: limit - 1] + "…") if len(text) > limit else text or "—"


def location(item):
    """`file:line` for a finding, or `—` when neither is present."""
    path = pick(item, FILE_KEYS)
    line = pick(item, LINE_KEYS)
    if path and line:
        return "%s:%s" % (path, line)
    return str(path or line or "—")


def severity_rank(item):
    sev = str(pick(item, SEVERITY_KEYS, "")).lower()
    return SEVERITY_ORDER.index(sev) if sev in SEVERITY_ORDER else len(SEVERITY_ORDER)


def suggestions_of(item):
    """Code suggestions attached to a finding, as a list of strings.

    The CLI sends `suggestions` as a list of code blobs, but tolerate a bare
    string and dict-wrapped shapes so an unfamiliar payload still renders.
    """
    raw = pick(item, SUGGESTION_KEYS, [])
    if isinstance(raw, str):
        raw = [raw]
    if not isinstance(raw, list):
        return []
    out = []
    for entry in raw:
        if isinstance(entry, str):
            text = entry
        elif isinstance(entry, dict):
            text = str(pick(entry, ("code", "text", "content", "patch", "diff")))
        else:
            text = str(entry)
        if text.strip():
            out.append(text.rstrip())
    return out


def fence(code, lang=""):
    """Wrap code in a fence long enough to survive backticks inside it.

    Suggestions are real source, and markdown/docs suggestions routinely
    contain ``` themselves -- a fixed three-backtick fence would end early and
    spill the rest of the block into the page as prose.
    """
    longest = 0
    for run in re.findall(r"`+", code):
        longest = max(longest, len(run))
    bar = "`" * max(3, longest + 1)
    return ["%s%s" % (bar, lang), code, bar]


def lang_for(path):
    """Best-effort fence language, purely for syntax highlighting."""
    ext = str(path).rsplit(".", 1)[-1].lower() if "." in str(path) else ""
    return {
        "ts": "ts", "tsx": "tsx", "js": "js", "jsx": "jsx",
        "py": "python", "go": "go", "rs": "rust", "rb": "ruby",
        "java": "java", "kt": "kotlin", "cs": "csharp", "php": "php",
        "sh": "bash", "bash": "bash", "yml": "yaml", "yaml": "yaml",
        "json": "json", "md": "markdown", "sql": "sql", "css": "css",
        "html": "html", "toml": "toml",
    }.get(ext, "")


def anchor(text):
    """GitHub's heading-anchor slug: lowercase, punctuation dropped, spaces hyphenated."""
    slug = re.sub(r"[^\w\s-]", "", str(text).lower())
    return re.sub(r"[\s_]+", "-", slug).strip("-")


def heading(index, item):
    """The `###` text for a finding. Shared with `anchor` so links can't drift."""
    return "%d. `%s` — %s" % (
        index,
        location(item),
        str(pick(item, SEVERITY_KEYS, "unspecified")),
    )


def render_finding(index, item):
    """Full, untruncated markdown for one finding.

    Deliberately NOT a table row: findings run to several hundred characters of
    prose plus multi-line code suggestions, and a markdown cell can hold
    neither -- newlines collapse and anything past the cell limit is lost.
    """
    lines = [
        "### %s" % heading(index, item),
        "",
        redact(describe(item)),
        "",
    ]
    for code in suggestions_of(item):
        lines += ["**Suggested change**", ""]
        lines += fence(redact(code), lang_for(pick(item, FILE_KEYS)))
        lines += [""]
    return lines


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


# GitHub rejects a step summary larger than 1 MiB -- and rejects the *whole*
# file, so an oversized raw-JSON appendix would take the findings down with it.
SUMMARY_LIMIT = 1024 * 1024


def write_summary(lines):
    path = os.environ.get("GITHUB_STEP_SUMMARY")
    text = "\n".join(lines) + "\n"
    if not path:
        sys.stdout.write(text)
        return
    if len(text.encode("utf-8")) > SUMMARY_LIMIT:
        text = trim_summary(lines)
    with open(path, "a") as handle:
        handle.write(text)


def trim_summary(lines):
    """Drop the raw-JSON appendix to fit the summary limit; keep the findings.

    The appendix is a convenience duplicate of `review-file`, so it is the only
    part safe to lose. If the findings alone still exceed the limit they are
    cut at a line boundary with an explicit note -- never silently.
    """
    try:
        start = lines.index("<details><summary>Raw review JSON</summary>")
        kept = lines[:start] + [
            "_Raw JSON omitted: the summary hit GitHub's 1 MiB limit. "
            "Use the `review-file` output for the full payload._"
        ]
    except ValueError:
        kept = list(lines)

    text = "\n".join(kept) + "\n"
    if len(text.encode("utf-8")) <= SUMMARY_LIMIT:
        return text

    note = (
        "\n\n_Summary truncated at GitHub's 1 MiB limit. "
        "Use the `review-file` output for all findings._\n"
    )
    budget = SUMMARY_LIMIT - len(note.encode("utf-8"))
    out = []
    used = 0
    for line in kept:
        size = len(line.encode("utf-8")) + 1
        if used + size > budget:
            break
        out.append(line)
        used += size
    return "\n".join(out) + note


def echo_result(findings, status):
    """Print the outcome to stdout as well as the job summary.

    The summary renders on the run's Summary page, which is not where anyone
    looks when a step seems to have done nothing -- without this the Summarize
    step logs literally nothing and the review result is invisible in the logs.
    """
    if findings is None:
        print("status=%s findings=unknown (unrecognized output shape)" % status)
        return

    print("status=%s findings=%d" % (status, len(findings)))
    if not findings:
        print("No findings.")
        return

    # Every finding, in full. These run to several hundred characters and carry
    # code suggestions; clipping them to one short line made the logs useless
    # for actually reading the review.
    for number, item in enumerate(sorted(findings, key=severity_rank), 1):
        print("")
        print(
            "%d. [%s] %s"
            % (number, str(pick(item, SEVERITY_KEYS, "?")), location(item))
        )
        for line in redact(describe(item)).splitlines() or [""]:
            print("   %s" % line)
        for code in suggestions_of(item):
            print("   --- suggested change ---")
            for line in redact(code).splitlines():
                print("   %s" % line)


def main():
    review_file = sys.argv[1] if len(sys.argv) > 1 else "review.json"
    try:
        with open(review_file) as handle:
            payload = json.load(handle)
    except (OSError, ValueError) as exc:
        write_output("status", "error")
        write_output("finding-count", "unknown")
        print("status=error: could not read review output: %s" % exc)
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

    status = str(payload.get("status", "ok")) if isinstance(payload, dict) else "ok"
    echo_result(findings, status)

    write_output("status", status)
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

    ordered = sorted(findings, key=severity_rank)

    tally = {}
    for item in ordered:
        key = str(pick(item, SEVERITY_KEYS, "unspecified"))
        tally[key] = tally.get(key, 0) + 1
    breakdown = ", ".join("%d %s" % (tally[k], k) for k in tally)

    lines += [
        "**%d finding%s** — %s"
        % (len(ordered), "" if len(ordered) == 1 else "s", breakdown),
        "",
        "| # | Location | Severity |",
        "| --- | --- | --- |",
    ]
    # An index only: every finding is then written out in full below, so
    # nothing here needs to carry the comment text.
    for number, item in enumerate(ordered, 1):
        lines.append(
            "| [%d](#%s) | `%s` | %s |"
            % (
                number,
                anchor(heading(number, item)),
                cell(location(item), 120),
                cell(pick(item, SEVERITY_KEYS), 20),
            )
        )

    lines += [""]
    for number, item in enumerate(ordered, 1):
        lines += render_finding(number, item)

    lines += [
        "<details><summary>Raw review JSON</summary>",
        "",
        "```json",
        raw_json,
        "```",
        "",
        "</details>",
    ]
    write_summary(lines)
    return 0


if __name__ == "__main__":
    sys.exit(main())
