#!/bin/sh
# Dual contract:
#   serve        -> start the HTTP API (primary interface)
#   <anything>   -> pass straight through to the coderabbit CLI
#
# This is the container's entrypoint override -- there is no Dockerfile. It is
# bind-mounted into a stock image along with init.sh and server.py.
#
# Provisioning may need root (creating the runner user), then we drop to the
# unprivileged `runner` user for the actual work.
set -eu

INSTALL_DIR="${CODERABBIT_INSTALL_DIR:-/opt/cr/bin}"
RR_DIR="${RR_DIR:-/opt/rr}"
SERVER_PY="${RR_SERVER:-/opt/server/server.py}"
RUN_UID="${RR_UID:-1000}"
RUN_GID="${RR_GID:-1000}"

# Provisioning is a sibling of this script inside the mounted docker/ directory.
"$RR_DIR/init.sh"

# Drop privileges when we start as root. setpriv keeps this a single exec with
# no intermediate shell owning the signal handling; fall back to su if the image
# lacks util-linux.
run_as_runner() {
    if [ "$(id -u)" != "0" ]; then
        exec "$@"
    fi
    if command -v setpriv >/dev/null 2>&1; then
        exec setpriv --reuid="$RUN_UID" --regid="$RUN_GID" --init-groups --inh-caps=-all "$@"
    fi
    if command -v su >/dev/null 2>&1 && id -u runner >/dev/null 2>&1; then
        exec su -s /bin/sh runner -c 'exec "$0" "$@"' -- "$@"
    fi
    echo "[entrypoint] WARNING: cannot drop privileges; running as root" >&2
    exec "$@"
}

# HOME must point at the runner's own directory: the CLI writes config and cache
# there, and root's HOME is not writable after the drop.
if [ "$(id -u)" = "0" ] && [ -d /home/runner ]; then
    HOME=/home/runner
    export HOME
fi

case "${1:-serve}" in
    serve)
        run_as_runner python3 "$SERVER_PY"
        ;;
    *)
        run_as_runner "$INSTALL_DIR/coderabbit" "$@"
        ;;
esac
