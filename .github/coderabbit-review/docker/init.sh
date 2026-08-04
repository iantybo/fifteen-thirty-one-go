#!/bin/sh
# Runtime provisioning of the CodeRabbit CLI.
#
# There is no Dockerfile in this project. We run a stock upstream image
# (python:3.12-bookworm by default) and provision it at container start, so the
# CLI version floats without an image build. That image already ships git, curl,
# ca-certificates, unzip, setpriv, and the full Python stdlib, so the only thing
# installed here is the CLI itself -- no apt on the startup path.
#
# apt is still used as a fallback if a caller points RR_IMAGE at a leaner base.
#
# Idempotent: returns early when already provisioned, so warm container reuse is
# cheap. Requires root when it needs to create the runner user or apt-install a
# fallback dep; the entrypoint drops privileges afterwards.
set -eu

INSTALL_DIR="${CODERABBIT_INSTALL_DIR:-/opt/cr/bin}"
STATE_FILE="${CR_INIT_STATE_FILE:-/tmp/cr-init.json}"
RUN_UID="${RR_UID:-1000}"
RUN_GID="${RR_GID:-1000}"

log() { printf '[init] %s\n' "$*" >&2; }

write_state() {
    # Consumed by the API's /healthz so it can report provisioned versions.
    cat >"$STATE_FILE" <<EOF
{"status":"ok","git":"$1","coderabbit":"$2"}
EOF
    chmod 0644 "$STATE_FILE"
}

# --- unprivileged user ------------------------------------------------------
# The stock image runs as root and has no `runner` user, so create one for the
# entrypoint to drop to. Skipped when we are already unprivileged.
ensure_runner_user() {
    [ "$(id -u)" = "0" ] || return 0
    if id -u runner >/dev/null 2>&1; then
        return 0
    fi
    log "creating unprivileged runner user (uid $RUN_UID)"
    if command -v groupadd >/dev/null 2>&1; then
        groupadd --gid "$RUN_GID" runner 2>/dev/null || true
        useradd --uid "$RUN_UID" --gid "$RUN_GID" --create-home --shell /bin/sh runner 2>/dev/null || true
    else
        addgroup -g "$RUN_GID" runner 2>/dev/null || true
        adduser -u "$RUN_UID" -G runner -D -s /bin/sh runner 2>/dev/null || true
    fi
}

# --- fallback deps ----------------------------------------------------------
# No-ops on the default image; only fires if someone swaps in a leaner base.
ensure_deps() {
    missing=""
    for tool in curl git unzip; do
        command -v "$tool" >/dev/null 2>&1 || missing="$missing $tool"
    done
    [ -n "$missing" ] || return 0

    if [ "$(id -u)" != "0" ]; then
        log "FATAL: missing deps ($missing) and not root, cannot install"
        exit 1
    fi
    log "installing missing deps:$missing"
    apt-get update >/dev/null
    # shellcheck disable=SC2086
    apt-get install -y --no-install-recommends ca-certificates $missing >/dev/null
    rm -rf /var/lib/apt/lists/*
}

# --- early return if already provisioned ------------------------------------
if [ -x "$INSTALL_DIR/coderabbit" ] \
    && "$INSTALL_DIR/coderabbit" --version >/dev/null 2>&1 \
    && command -v git >/dev/null 2>&1; then
    log "already provisioned; skipping install"
    write_state \
        "$(git --version 2>/dev/null | head -n1)" \
        "$("$INSTALL_DIR/coderabbit" --version 2>/dev/null | head -n1)"
    exit 0
fi

ensure_runner_user
ensure_deps

mkdir -p "$INSTALL_DIR"

# --- CodeRabbit CLI ---------------------------------------------------------
log "installing CodeRabbit CLI into $INSTALL_DIR"
# CODERABBIT_VERSION (optional) pins a version; CI=true suppresses the
# post-install interactive login prompt. Both are read from the environment by
# the install script itself.
curl -fsSL https://cli.coderabbit.ai/install.sh | sh >&2

# The install script may land the binary in its default ~/.local/bin when
# CODERABBIT_INSTALL_DIR is not honored; link it into place if so.
if [ ! -x "$INSTALL_DIR/coderabbit" ] && [ -x "$HOME/.local/bin/coderabbit" ]; then
    log "linking from $HOME/.local/bin into $INSTALL_DIR"
    ln -sf "$HOME/.local/bin/coderabbit" "$INSTALL_DIR/coderabbit"
    [ -e "$HOME/.local/bin/cr" ] && ln -sf "$HOME/.local/bin/cr" "$INSTALL_DIR/cr"
fi

# The runner user must be able to execute the CLI and write its own config/cache
# under $HOME after the privilege drop.
if [ "$(id -u)" = "0" ]; then
    chown -R "$RUN_UID:$RUN_GID" "$INSTALL_DIR" 2>/dev/null || true
    [ -d /home/runner ] && chown -R "$RUN_UID:$RUN_GID" /home/runner 2>/dev/null || true
    # The install script runs as root, so a real (non-symlink) binary under
    # root's HOME would be unreadable post-drop; fix that up too.
    [ -d "$HOME/.local" ] && chmod -R a+rX "$HOME/.local" 2>/dev/null || true
fi

# --- verify -----------------------------------------------------------------
# Fail loudly rather than letting the server start and return confusing 502s.
if ! command -v git >/dev/null 2>&1; then
    log "FATAL: git not available after install"
    exit 1
fi
if [ ! -x "$INSTALL_DIR/coderabbit" ]; then
    log "FATAL: coderabbit binary not found at $INSTALL_DIR/coderabbit"
    exit 1
fi
if ! "$INSTALL_DIR/coderabbit" --version >/dev/null 2>&1; then
    log "FATAL: coderabbit binary present but not runnable"
    exit 1
fi

GIT_VERSION="$(git --version | head -n1)"
CR_VERSION="$("$INSTALL_DIR/coderabbit" --version 2>/dev/null | head -n1)"
log "provisioned: $GIT_VERSION / $CR_VERSION"
write_state "$GIT_VERSION" "$CR_VERSION"
