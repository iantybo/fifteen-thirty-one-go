#!/usr/bin/env python3
"""HTTP API around `coderabbit review --agent`.

Stdlib only, on purpose: nothing to pip install at container start.

Security posture, stated plainly: this API has NO authentication of its own. It
binds loopback by default; set BIND_ALL=1 only behind an external auth layer.
"""

import json
import os
import re
import shutil
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

CODERABBIT = os.environ.get("CODERABBIT_INSTALL_DIR", "/opt/cr/bin") + "/coderabbit"
STATE_FILE = os.environ.get("CR_INIT_STATE_FILE", "/tmp/cr-init.json")

DEFAULT_TIMEOUT = 1800
MAX_TIMEOUT = 7200
MAX_BODY_BYTES = 1 << 20

# The CLI operates on a shared working tree, so concurrent reviews would race.
_review_lock = threading.Lock()


def redact(text, *secrets):
    """Strip API keys from anything we return or log.

    The per-request key override means keys reach this process; they must never
    reach a response body, a log line, or the job summary.
    """
    if not text:
        return text
    for secret in secrets:
        if secret:
            text = text.replace(secret, "***redacted***")
    # Belt and braces: catch any cr-prefixed token we were not handed directly.
    return re.sub(r"\bcr-[A-Za-z0-9_\-]{8,}", "***redacted***", text)


def parse_agent_output(stdout):
    """Parse `review --agent` stdout into (events, parse_error).

    The CLI emits JSON Lines -- one object per line, interleaving progress
    (`review_context`, `status`, `heartbeat`) with `finding` objects -- NOT a
    single JSON document. Verified against CLI 0.7.1.

    Falls back to a whole-document parse so a future CLI that emits one JSON
    object still works; a single object is normalized to a one-event list.
    """
    events = []
    bad_lines = 0
    for line in stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            events.append(json.loads(line))
        except ValueError:
            bad_lines += 1

    if events and not bad_lines:
        return events, None

    # Not line-delimited (or partially garbled): try the whole blob.
    try:
        whole = json.loads(stdout)
    except (ValueError, TypeError):
        if events:
            # Some lines parsed; keep them rather than discarding a good review.
            return events, None
        return None, "CLI did not emit valid JSON on stdout"
    return (whole if isinstance(whole, list) else [whole]), None


def extract_findings(events):
    """Pull finding objects out of the event stream."""
    findings = []
    for event in events:
        if not isinstance(event, dict):
            continue
        if event.get("type") == "finding":
            findings.append(event)
            continue
        # Tolerate a batched shape, e.g. {"findings": [...]}.
        for key in ("findings", "comments", "issues", "review_comments", "results"):
            value = event.get(key)
            if isinstance(value, list):
                findings.extend(v for v in value if isinstance(v, dict))
                break
    return findings


def first_error_event(events):
    """Return the first event describing a CLI error, if any."""
    for event in events:
        if isinstance(event, dict) and event.get("type") == "error":
            return event
    return None


def build_argv(body):
    """Map request fields onto CLI flags.

    Built as an argv list and never passed through a shell, so hostile values in
    e.g. `base` cannot inject commands.
    """
    argv = [CODERABBIT, "review", "--agent"]

    mode = body.get("mode")
    if mode == "committed":
        argv.append("--committed")
    elif mode == "uncommitted":
        argv.append("--uncommitted")
    elif mode not in (None, "", "auto"):
        raise ValueError("mode must be one of: committed, uncommitted, auto")

    base = body.get("base")
    if base:
        if not isinstance(base, str):
            raise ValueError("base must be a string")
        argv += ["--base", base]

    if body.get("include_untracked"):
        argv.append("--include-untracked")
    if body.get("light"):
        argv.append("--light")

    extra = body.get("extra_args") or []
    if extra:
        if not isinstance(extra, list) or not all(isinstance(a, str) for a in extra):
            raise ValueError("extra_args must be a list of strings")
        argv += extra

    return argv


def authenticate(api_key, env):
    """Non-interactive login. Returns (0, "") on success, (1, detail) on failure.

    Two gotchas, both verified against CLI 0.7.1:
      - `auth login --api-key` exits 0 even when the key is rejected, so the
        exit code alone cannot be trusted; the output text must be inspected.
      - The key must be passed as a flag here (the env var does not drive this
        subcommand), so the argv is deliberately never logged.
    """
    try:
        proc = subprocess.run(
            [CODERABBIT, "auth", "login", "--api-key", api_key],
            env=env,
            capture_output=True,
            text=True,
            timeout=120,
        )
    except subprocess.TimeoutExpired:
        return 1, "authentication timed out after 120s"
    except OSError as exc:
        return 1, "failed to run auth login: %s" % exc

    combined = "%s\n%s" % (proc.stdout or "", proc.stderr or "")
    lowered = combined.lower()
    failed = proc.returncode != 0 or any(
        marker in lowered
        for marker in (
            "authentication failed",
            "invalid or expired",
            "unauthorized",
            "environment_unsupported",
        )
    )
    if failed:
        return 1, combined.strip()[:2000]
    return 0, ""


def run_review(body):
    """Execute the review. Returns (http_status, response_dict)."""
    workdir = body.get("workdir") or "/workspace"
    if not isinstance(workdir, str):
        return 400, {"status": "error", "error": "workdir must be a string"}
    if not os.path.isdir(workdir):
        return 400, {"status": "error", "error": "workdir does not exist: %s" % workdir}
    if not os.path.exists(os.path.join(workdir, ".git")):
        return 400, {
            "status": "error",
            "error": (
                "workdir is not a git repository: %s. Mount the repo at this path "
                "and ensure .git is present (actions/checkout needs fetch-depth: 0 "
                "for --base diffs)." % workdir
            ),
        }

    api_key = body.get("api_key") or os.environ.get("CODERABBIT_API_KEY")
    if not api_key:
        return 401, {
            "status": "error",
            "error": (
                "no API key: set CODERABBIT_API_KEY on the container or pass "
                "api_key in the request body"
            ),
        }

    try:
        timeout = int(body.get("timeout_seconds") or DEFAULT_TIMEOUT)
    except (TypeError, ValueError):
        return 400, {"status": "error", "error": "timeout_seconds must be an integer"}
    timeout = max(1, min(timeout, MAX_TIMEOUT))

    try:
        argv = build_argv(body)
    except ValueError as exc:
        return 400, {"status": "error", "error": str(exc)}

    if not os.path.exists(CODERABBIT):
        return 503, {
            "status": "error",
            "error": "coderabbit CLI not provisioned at %s" % CODERABBIT,
        }

    # Key goes via the environment, never argv, so it cannot leak through `ps`.
    env = dict(os.environ, CODERABBIT_API_KEY=api_key, CI="true")

    # CODERABBIT_API_KEY alone is NOT enough: `review --agent` otherwise attempts
    # an interactive browser login and fails with
    # {"type":"error","phase":"auth","status":"environment_unsupported"}.
    # An explicit non-interactive login is required first. Verified against CLI 0.7.1.
    auth_status, auth_error = authenticate(api_key, env)
    if auth_status != 0:
        return 401, {
            "status": "error",
            "error": "CodeRabbit authentication failed",
            "detail": redact(auth_error, api_key),
        }

    if not _review_lock.acquire(blocking=False):
        return 409, {
            "status": "error",
            "error": "a review is already in flight in this container",
        }
    try:
        proc = subprocess.run(
            argv,
            cwd=workdir,
            env=env,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return 504, {
            "status": "error",
            "error": "review exceeded timeout of %ds" % timeout,
        }
    except OSError as exc:
        return 500, {"status": "error", "error": "failed to run CLI: %s" % exc}
    finally:
        _review_lock.release()

    stdout = redact(proc.stdout, api_key)
    stderr = redact(proc.stderr, api_key)

    # Parse the REDACTED stdout, not proc.stdout: the CLI can echo the key back
    # inside its own output, and parsing the raw text would put it straight into
    # the response body (and from there into GHA outputs and the job summary).
    events, parse_error = parse_agent_output(stdout)
    if parse_error:
        # Return the raw output rather than an empty success — otherwise these
        # failures are undebuggable.
        return 502, {
            "status": "error",
            "error": parse_error,
            "exit_code": proc.returncode,
            "raw_stdout": stdout[:100_000],
            "stderr": stderr[:20_000],
        }

    # The CLI can emit well-formed JSON *describing a failure* (e.g.
    # {"type":"error","phase":"auth",...}). Parsing success alone is not review
    # success -- reporting 200/ok here would make a failed review look clean.
    error_event = first_error_event(events)
    if proc.returncode != 0 or error_event:
        return 502, {
            "status": "error",
            "error": "coderabbit review failed",
            "exit_code": proc.returncode,
            "detail": error_event,
            "review": events,
            "stderr": stderr[:20_000],
        }

    findings = extract_findings(events)
    return 200, {
        "status": "ok",
        "exit_code": proc.returncode,
        "finding_count": len(findings),
        "findings": findings,
        "review": events,
        "stderr": stderr[:20_000],
        "argv": list(argv),
    }


def read_state():
    try:
        with open(STATE_FILE) as handle:
            return json.load(handle)
    except (OSError, ValueError):
        return {}


class Handler(BaseHTTPRequestHandler):
    server_version = "remote-review/1.0"

    def log_message(self, fmt, *args):
        # Method + path + status only. Never request bodies: they may hold keys.
        sys.stderr.write("[api] %s\n" % (fmt % args))

    def _respond(self, status, payload):
        body = json.dumps(payload, indent=2).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.split("?")[0] in ("/healthz", "/health"):
            state = read_state()
            ready = bool(state.get("coderabbit")) and os.path.exists(CODERABBIT)
            self._respond(
                200 if ready else 503,
                {
                    "status": "ok" if ready else "provisioning",
                    "git": state.get("git"),
                    "coderabbit": state.get("coderabbit"),
                    "git_path": shutil.which("git"),
                    "review_in_flight": _review_lock.locked(),
                    "api_key_from_env": bool(os.environ.get("CODERABBIT_API_KEY")),
                },
            )
            return
        self._respond(404, {"status": "error", "error": "not found: %s" % self.path})

    def do_POST(self):
        if self.path.split("?")[0] != "/review":
            self._respond(404, {"status": "error", "error": "not found: %s" % self.path})
            return

        try:
            length = int(self.headers.get("Content-Length") or 0)
        except ValueError:
            self._respond(400, {"status": "error", "error": "bad Content-Length"})
            return
        if length > MAX_BODY_BYTES:
            self._respond(413, {"status": "error", "error": "request body too large"})
            return

        raw = self.rfile.read(length) if length else b"{}"
        try:
            body = json.loads(raw or b"{}")
        except ValueError:
            self._respond(400, {"status": "error", "error": "body is not valid JSON"})
            return
        if not isinstance(body, dict):
            self._respond(400, {"status": "error", "error": "body must be a JSON object"})
            return

        header_key = self.headers.get("X-CodeRabbit-Api-Key")
        if header_key and not body.get("api_key"):
            body["api_key"] = header_key

        status, payload = run_review(body)
        self._respond(status, payload)


def main():
    port = int(os.environ.get("PORT", "8080"))
    # Loopback unless explicitly opened up: the default posture should not be an
    # unauthenticated review endpoint on every interface.
    host = "0.0.0.0" if os.environ.get("BIND_ALL") == "1" else "127.0.0.1"
    server = ThreadingHTTPServer((host, port), Handler)
    state = read_state()
    sys.stderr.write(
        "[api] listening on %s:%d (git=%s coderabbit=%s)\n"
        % (host, port, state.get("git"), state.get("coderabbit"))
    )
    if host == "0.0.0.0":
        sys.stderr.write(
            "[api] WARNING: bound to all interfaces with no authentication; "
            "put an auth layer in front of this\n"
        )
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
