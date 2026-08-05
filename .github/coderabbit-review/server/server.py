#!/usr/bin/env python3
"""HTTP API around `coderabbit review --agent`.

Stdlib only, on purpose: nothing to pip install at container start.

Security posture, stated plainly: this API has NO authentication of its own. It
binds loopback by default; set BIND_ALL=1 only behind an external auth layer.
"""

import ipaddress
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

CODERABBIT = os.environ.get("CODERABBIT_INSTALL_DIR", "/opt/cr/bin") + "/coderabbit"
STATE_FILE = os.environ.get("CR_INIT_STATE_FILE", "/tmp/cr-init.json")

DEFAULT_TIMEOUT = 1800
MAX_TIMEOUT = 7200
MAX_BODY_BYTES = 1 << 20

# Server-side clone. Cloning is opt-in per request (nothing happens without
# repo_url); RR_ALLOW_CLONE=0 is the hard kill switch for a locked-down deploy.
CLONE_ROOT = os.environ.get("RR_CLONE_ROOT", "/tmp/rr-clones")
CLONE_TIMEOUT = int(os.environ.get("RR_CLONE_TIMEOUT", "600"))
MAX_CLONE_TIMEOUT = 1800
CLONE_MAX_KEPT = int(os.environ.get("RR_CLONE_MAX_KEPT", "3"))
ALLOW_CLONE = os.environ.get("RR_ALLOW_CLONE", "1") != "0"
KEEP_CLONES = os.environ.get("RR_KEEP_CLONES") == "1"
CLONE_HOSTS = os.environ.get("RR_CLONE_HOSTS", "github.com,gitlab.com,bitbucket.org")
MIN_FREE_BYTES = 1 << 30  # 1 GiB

# extra_args appends arbitrary flags to the CLI invocation. Safe by construction
# against injection (argv list, never shell-interpolated), but with clone support
# the caller controls the repo *and* the flags, so a path-taking flag becomes a
# file-read primitive. Default on to preserve the documented passthrough; run.sh
# turns it off when it puts the API on a public tunnel.
ALLOW_EXTRA_ARGS = os.environ.get("RR_ALLOW_EXTRA_ARGS", "1") != "0"

# The CLI operates on a shared working tree, so concurrent reviews would race.
_review_lock = threading.Lock()

# A GitHub PAT does not match the cr- pattern, so without these a failed clone
# would echo the token straight back to the caller in a 400/502 body.
GITHUB_TOKEN_RE = re.compile(
    r"\b(gh[pousr]_[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{20,})"
)
CRED_URL_RE = re.compile(r"(https?://)[^/\s:@]+:[^/\s@]+@")


def redact(text, *secrets):
    """Strip API keys and git credentials from anything we return or log.

    The per-request key override means keys reach this process; they must never
    reach a response body, a log line, or the job summary.
    """
    if not text:
        return text
    for secret in secrets:
        if secret:
            text = text.replace(secret, "***redacted***")
    # URL credentials first: this also catches non-GitHub tokens, which no
    # token-shaped pattern would match.
    text = CRED_URL_RE.sub(r"\1***redacted***@", text)
    text = GITHUB_TOKEN_RE.sub("***redacted***", text)
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


def validate_repo_url(url):
    """Validate a clone URL. Returns (url, error).

    repo_url is attacker-controlled whenever this API is reachable, so this is
    the SSRF boundary. Verified behaviours it defends against, all tested:
      - `file:///etc/passwd` and a bare `/tmp/x` path clone successfully
      - `ext::sh -c whoami` is a remote-helper RCE (git blocks it by default,
        but only while the scheme allowlist keeps it unreachable)
      - `https://github.com.evil.com/...` is a distinct host, not a subdomain
    Credentials are rejected outright so a token can never land in .git/config;
    it travels via GIT_ASKPASS instead.
    """
    if not isinstance(url, str):
        return None, "repo_url must be a string"
    if not url or len(url) > 2048:
        return None, "repo_url must be 1..2048 characters"
    if any(ch in url for ch in "\x00\n\r\t"):
        return None, "repo_url contains control characters"
    if url.startswith("-"):
        # Would be read as a flag by git.
        return None, "repo_url must not start with '-'"

    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != "https":
        return None, (
            "repo_url must use https (got %r). file://, git://, ssh://, ext:: and "
            "bare paths are refused." % (parsed.scheme or "no scheme")
        )
    if parsed.username or parsed.password:
        return None, (
            "repo_url must not embed credentials; pass github_token instead"
        )
    host = (parsed.hostname or "").lower()
    if not host:
        return None, "repo_url has no host"
    try:
        if parsed.port not in (None, 443):
            return None, "repo_url must use the default https port"
    except ValueError:
        return None, "repo_url has an invalid port"

    allowed = [h.strip().lower() for h in CLONE_HOSTS.split(",") if h.strip()]
    if not host_allowed(host, allowed):
        return None, "repo_url host %r is not allowed (allowed: %s)" % (
            host,
            ", ".join(allowed) or "none",
        )
    return url, None


def host_allowed(host, allowed):
    """Exact match, or a `*.example.com` suffix entry.

    Exact matching is what stops `github.com.evil.com`, which shares a prefix
    with an allowlisted host but is an entirely different domain.
    """
    for entry in allowed:
        if entry.startswith("*."):
            if host == entry[2:] or host.endswith(entry[1:]):
                return True
        elif host == entry:
            return True
    return False


def check_host_public(host):
    """Resolve host and require every address be globally routable.

    Defence in depth behind the host allowlist: blocks loopback, link-local and
    cloud metadata (169.254.169.254) should an allowlist entry ever be loosened.

    Deliberately separate from validate_repo_url so URL parsing stays testable
    without DNS. Inherently TOCTOU -- git re-resolves independently -- so the
    exact-host allowlist, not this, is the real control.
    """
    try:
        infos = socket.getaddrinfo(host, 443, proto=socket.IPPROTO_TCP)
    except OSError as exc:
        return False, "cannot resolve %s: %s" % (host, exc)
    if not infos:
        return False, "cannot resolve %s" % host
    for info in infos:
        addr = info[4][0]
        try:
            ip = ipaddress.ip_address(addr)
        except ValueError:
            return False, "unparseable address for %s" % host
        if not ip.is_global:
            return False, "%s resolves to non-public address %s" % (host, addr)
    return True, None


def validate_ref(ref, field="ref"):
    """Validate a git ref name. Returns (ref, error)."""
    if ref is None or ref == "":
        return None, None
    if not isinstance(ref, str):
        return None, "%s must be a string" % field
    if len(ref) > 255:
        return None, "%s must be at most 255 characters" % field
    if any(ch in ref for ch in "\x00\n\r\t "):
        return None, "%s contains whitespace or control characters" % field
    if ref.startswith("-"):
        # Otherwise git reads it as a flag, e.g. --upload-pack=...
        return None, "%s must not start with '-'" % field
    if ref.startswith("/") or ".." in ref:
        return None, "%s must not be an absolute path or contain '..'" % field
    return ref, None


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
        if not ALLOW_EXTRA_ARGS:
            raise ValueError(
                "extra_args is disabled on this container "
                "(set RR_ALLOW_EXTRA_ARGS=1 to enable)"
            )
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


def check_git_usable(workdir):
    """Verify git can actually operate on workdir. Returns (ok, error_message).

    Exists because `.git` being present says nothing about git being willing to
    use it -- see the safe.directory handling in docker/init.sh.
    """
    try:
        proc = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            cwd=workdir,
            capture_output=True,
            text=True,
            timeout=30,
        )
    except subprocess.TimeoutExpired:
        return False, "git rev-parse timed out in %s" % workdir
    except OSError as exc:
        return False, "failed to run git: %s" % exc

    if proc.returncode == 0:
        return True, None

    detail = (proc.stderr or proc.stdout or "").strip()[:1000]
    if "dubious ownership" in detail:
        try:
            owner = os.stat(workdir).st_uid
        except OSError:
            owner = "?"
        return False, (
            "git refuses %s: it is owned by uid %s but this process runs as uid "
            "%d. Add it to safe.directory (docker/init.sh does this at startup) "
            "or run the container with a matching --user. git said: %s"
            % (workdir, owner, os.getuid(), detail)
        )
    return False, "git cannot use %s: %s" % (workdir, detail)


def write_askpass(dirpath):
    """Write a GIT_ASKPASS helper and return its path.

    The token is read from the subprocess environment, never baked into the
    script body -- so it is never written to disk, and never appears in the
    remote URL, .git/config, or `ps` output either.
    """
    path = os.path.join(dirpath, "askpass.sh")
    with open(path, "w") as handle:
        handle.write(
            '#!/bin/sh\n'
            '# Username prompt gets a literal; password prompt gets the token.\n'
            'case "$1" in\n'
            '  Username*) printf %s\\\\n "x-access-token" ;;\n'
            '  *) printf %s\\\\n "$GIT_TOKEN" ;;\n'
            'esac\n'
        )
    os.chmod(path, 0o700)
    return path


def git_run(argv, timeout, env, cwd=None):
    """Run a git command, returning (proc, error). Never raises."""
    try:
        proc = subprocess.run(
            argv, cwd=cwd, env=env, capture_output=True, text=True, timeout=timeout
        )
        return proc, None
    except subprocess.TimeoutExpired:
        return None, "timeout"
    except OSError as exc:
        return None, "oserror: %s" % exc


def prune_clones(keep):
    """Keep only the newest `keep` clone trees under CLONE_ROOT.

    Bounds disk regardless of the per-request cleanup, and reaps orphans left
    behind by a crashed request. Never raises: a prune failure must not turn a
    successful review into a 500.
    """
    try:
        entries = [
            os.path.join(CLONE_ROOT, name) for name in os.listdir(CLONE_ROOT)
        ]
        dirs = [p for p in entries if os.path.isdir(p)]
        dirs.sort(key=lambda p: os.path.getmtime(p), reverse=True)
        for stale in dirs[max(0, keep):]:
            shutil.rmtree(stale, ignore_errors=True)
    except OSError:
        pass


# git stderr fragments that mean "the remote refused our credentials" rather
# than "something broke". Drives the 401-vs-502 split.
_AUTH_FAILURE_MARKERS = (
    "authentication failed",
    "could not read username",
    "could not read password",
    "terminal prompts disabled",
    "invalid username or password",
    "http basic: access denied",
    "the requested url returned error: 403",
    "the requested url returned error: 401",
)


def clone_repo(url, ref, base, token, timeout):
    """Clone `url` into a fresh directory. Returns (workdir, info, status, error).

    On success status is None. On failure (workdir, info) are None and
    (status, error) describe the HTTP response.
    """
    try:
        os.makedirs(CLONE_ROOT, exist_ok=True)
        os.chmod(CLONE_ROOT, 0o700)
    except OSError as exc:
        return None, None, 500, "cannot create clone root: %s" % redact(str(exc), token)

    try:
        free = shutil.disk_usage(CLONE_ROOT).free
    except OSError:
        free = None
    if free is not None and free < MIN_FREE_BYTES:
        return None, None, 507, (
            "insufficient disk for a clone: %d MiB free at %s"
            % (free // (1 << 20), CLONE_ROOT)
        )

    # mkdtemp: 0700 and collision-free, with no path component derived from
    # user input.
    tmpdir = tempfile.mkdtemp(dir=CLONE_ROOT, prefix="c-")
    dest = os.path.join(tmpdir, "repo")

    askpass = write_askpass(tmpdir)
    env = dict(
        os.environ,
        GIT_ASKPASS=askpass,
        GIT_TERMINAL_PROMPT="0",  # else a private repo hangs until timeout
        GIT_ALLOW_PROTOCOL="https",  # second layer under validate_repo_url
        LC_ALL="C",
    )
    env["GIT_TOKEN"] = token or ""

    started = time.time()
    # A plain clone already populates refs/remotes/origin/*, so `--base main`
    # works with no extra fetch. Deliberately NOT --filter=blob:none: that sets
    # promisor=true, making the review lazily fetch blobs mid-run over the
    # network with credentials the review subprocess does not carry.
    # `--` terminates options so a hostile URL can never be read as a flag.
    proc, err = git_run(
        ["git", "-c", "protocol.version=2", "clone", "--quiet", "--no-tags",
         "--origin", "origin", "--", url, dest],
        timeout, env,
    )
    status, detail = _clone_outcome(proc, err, token, timeout, "clone")
    if status:
        shutil.rmtree(tmpdir, ignore_errors=True)
        return None, None, status, detail

    # A base that is a tag or non-branch ref may not be among the fetched
    # heads; fetch it explicitly so `--base` can resolve it.
    if base:
        check, _ = git_run(
            ["git", "rev-parse", "--verify", "--quiet", "origin/%s" % base],
            60, env, cwd=dest,
        )
        if not check or check.returncode != 0:
            git_run(
                ["git", "fetch", "--quiet", "--no-tags", "origin",
                 "+refs/heads/%s:refs/remotes/origin/%s" % (base, base)],
                timeout, env, cwd=dest,
            )

    if ref:
        # Resolve first. A caller naturally passes a branch name, which in a
        # fresh clone exists only as origin/<name>; a sha or tag resolves as-is.
        # Resolving up front gives a clear "ref not found" instead of git's
        # misleading "--detach does not take a path argument".
        target = None
        for candidate in (ref, "origin/%s" % ref):
            probe, _ = git_run(
                ["git", "rev-parse", "--verify", "--quiet", "%s^{commit}" % candidate],
                60, env, cwd=dest,
            )
            if probe and probe.returncode == 0:
                target = candidate
                break
        if target is None:
            shutil.rmtree(tmpdir, ignore_errors=True)
            return None, None, 502, (
                "ref %r not found in %s (tried %r and 'origin/%s')"
                % (ref, redact(url, token), ref, ref)
            )
        proc, err = git_run(
            ["git", "checkout", "--quiet", "--detach", target], timeout, env, cwd=dest
        )
        if (not proc) or proc.returncode != 0:
            status, detail = _clone_outcome(
                proc, err, token, timeout, "checkout of %r" % ref
            )
            shutil.rmtree(tmpdir, ignore_errors=True)
            return None, None, status or 502, detail

    info = {
        "repo_url": url,
        "host": urllib.parse.urlsplit(url).hostname,
        "ref": ref,
        "base": base,
        "resolved_sha": _rev_parse(dest, "HEAD", env),
        "base_resolved_sha": _rev_parse(dest, "origin/%s" % base, env) if base else None,
        "duration_ms": int((time.time() - started) * 1000),
    }
    if KEEP_CLONES:
        info["path"] = dest
    # The credential helper has done its job; it must not outlive the clone.
    # Removed here rather than only in run_review's finally, so the helper is
    # safe to call directly.
    try:
        os.remove(askpass)
    except OSError:
        pass
    return dest, info, None, None


def _rev_parse(cwd, rev, env):
    proc, _ = git_run(["git", "rev-parse", rev], 60, env, cwd=cwd)
    if proc and proc.returncode == 0:
        return proc.stdout.strip()
    return None


def _clone_outcome(proc, err, token, timeout, what):
    """Map a git failure onto (status, message). Returns (None, None) on success."""
    if err == "timeout":
        return 504, "%s exceeded timeout of %ds" % (what, timeout)
    if err:
        return 502, "failed to run git for %s: %s" % (what, redact(err, token))
    if proc.returncode == 0:
        return None, None
    stderr = redact((proc.stderr or proc.stdout or "").strip(), token)[:4000]
    low = stderr.lower()
    if any(marker in low for marker in _AUTH_FAILURE_MARKERS):
        # Deliberately hedged: GitHub answers a missing repo with the same auth
        # challenge as a private one, so it never leaks whether a repo exists.
        # Claiming "bad token" would send people chasing the wrong problem.
        return 401, (
            "the remote refused access to this repository. It may be private, may "
            "not exist, or the token may lack read access -- GitHub returns the "
            "same challenge for all three. Pass a valid github_token (or the "
            "X-GitHub-Token header). git said: %s" % stderr
        )
    return 502, "git %s failed: %s" % (what, stderr)


def run_review(body):
    """Execute the review. Returns (http_status, response_dict).

    Ordering matters. Everything cheap and local is validated first, outside the
    lock, so a malformed request never contends with a running review. The lock
    then covers authenticate + review as one critical section: `auth login`
    persists credentials under a single shared $HOME, so authenticating outside
    it let two requests with different keys interleave and review under each
    other's credentials.
    """
    # --- validation (no lock, no network) --------------------------------
    repo_url = body.get("repo_url")
    clone_ref = clone_base = None
    if repo_url is not None:
        if not ALLOW_CLONE:
            return 403, {
                "status": "error",
                "error": "cloning is disabled on this container (RR_ALLOW_CLONE=0)",
            }
        if body.get("workdir"):
            return 400, {
                "status": "error",
                "error": (
                    "workdir cannot be combined with repo_url: the clone defines "
                    "the working directory"
                ),
            }
        repo_url, err = validate_repo_url(repo_url)
        if err:
            return 400, {"status": "error", "error": err}
        clone_ref, err = validate_ref(body.get("ref"))
        if err:
            return 400, {"status": "error", "error": err}
        clone_base, err = validate_ref(body.get("base"), field="base")
        if err:
            return 400, {"status": "error", "error": err}

    try:
        clone_timeout = int(body.get("clone_timeout_seconds") or CLONE_TIMEOUT)
    except (TypeError, ValueError):
        return 400, {
            "status": "error",
            "error": "clone_timeout_seconds must be an integer",
        }
    clone_timeout = max(1, min(clone_timeout, MAX_CLONE_TIMEOUT))

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

    workdir = body.get("workdir") or "/workspace"
    if not isinstance(workdir, str):
        return 400, {"status": "error", "error": "workdir must be a string"}

    github_token = body.get("github_token")
    if github_token is not None and not isinstance(github_token, str):
        return 400, {"status": "error", "error": "github_token must be a string"}

    # Key goes via the environment, never argv, so it cannot leak through `ps`.
    # The GitHub token is deliberately NOT added here: it reaches git via a
    # per-clone env, so the review subprocess never inherits it.
    env = dict(os.environ, CODERABBIT_API_KEY=api_key, CI="true")

    # --- critical section -------------------------------------------------
    if not _review_lock.acquire(blocking=False):
        return 409, {
            "status": "error",
            "error": "a review is already in flight in this container",
        }
    clone_tmpdir = None
    clone_info = None
    try:
        if repo_url:
            # Resolution check sits here, not in validation: it is network I/O,
            # and it must not run before the lock is held.
            ok, host_err = check_host_public(urllib.parse.urlsplit(repo_url).hostname)
            if not ok:
                return 400, {"status": "error", "error": host_err}

            cloned, clone_info, cstatus, cerror = clone_repo(
                repo_url, clone_ref, clone_base, github_token, clone_timeout
            )
            if cstatus:
                return cstatus, {"status": "error", "error": cerror}
            workdir = cloned
            clone_tmpdir = os.path.dirname(cloned)

        if not os.path.isdir(workdir):
            return 400, {
                "status": "error",
                "error": "workdir does not exist: %s" % workdir,
            }
        if not os.path.exists(os.path.join(workdir, ".git")):
            return 400, {
                "status": "error",
                "error": (
                    "workdir is not a git repository: %s. Mount the repo at this path "
                    "and ensure .git is present (actions/checkout needs fetch-depth: 0 "
                    "for --base diffs)." % workdir
                ),
            }

        # A present .git is not enough: git refuses repositories owned by another
        # uid ("dubious ownership"), which the CLI then reports as the thoroughly
        # misleading "Git repository not found". Ask git itself so that case is
        # diagnosed here, with an actionable message, instead of as a late 502.
        git_ok, git_error = check_git_usable(workdir)
        if not git_ok:
            return 400, {"status": "error", "error": git_error}

        # CODERABBIT_API_KEY alone is NOT enough: `review --agent` otherwise attempts
        # an interactive browser login and fails with
        # {"type":"error","phase":"auth","status":"environment_unsupported"}.
        # An explicit non-interactive login is required first. Verified against CLI 0.7.1.
        auth_status, auth_error = authenticate(api_key, env)
        if auth_status != 0:
            return 401, {
                "status": "error",
                "error": "CodeRabbit authentication failed",
                "detail": redact(auth_error, api_key, github_token),
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

        stdout = redact(proc.stdout, api_key, github_token)
        stderr = redact(proc.stderr, api_key, github_token)

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
        payload = {
            "status": "ok",
            "exit_code": proc.returncode,
            "finding_count": len(findings),
            "findings": findings,
            "review": events,
            "stderr": stderr[:20_000],
            "argv": list(argv),
        }
        # Present only when a clone happened, so the mount-based response shape
        # stays byte-identical for the Action.
        if clone_info:
            payload["clone"] = clone_info
        return 200, payload
    finally:
        _review_lock.release()
        if clone_tmpdir and not KEEP_CLONES:
            shutil.rmtree(clone_tmpdir, ignore_errors=True)
        if clone_tmpdir:
            # Bound disk even when keeping is on, and reap orphaned trees.
            prune_clones(CLONE_MAX_KEPT)


def read_state():
    try:
        with open(STATE_FILE) as handle:
            return json.load(handle)
    except (OSError, ValueError):
        return {}


class Handler(BaseHTTPRequestHandler):
    server_version = "remote-review/1.0"

    def log_message(self, fmt, *args):
        # Method + path + status only. Never request bodies OR headers: both can
        # hold credentials (api_key/github_token, X-CodeRabbit-Api-Key/X-GitHub-Token).
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

        header_tok = self.headers.get("X-GitHub-Token")
        if header_tok and not body.get("github_token"):
            body["github_token"] = header_tok

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
