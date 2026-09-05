#!/bin/sh
set -eu

usage() {
	cat <<'EOF'
Usage: ./scripts/init-admin.sh

Interactively creates the first ROOT administrator by starting a temporary API
process with one-time bootstrap credentials. The target database must have no
active users, and MySQL plus Redis must already be available.

Optional environment variables:
  ADMIN_INIT_MANAGEMENT_URL     readiness URL base (default: http://127.0.0.1:9090)
  ADMIN_INIT_TIMEOUT_SECONDS    startup timeout (default: 60)
EOF
}

case "${1:-}" in
	-h|--help)
		usage
		exit 0
		;;
	"") ;;
	*)
		usage >&2
		exit 2
		;;
esac

if [ ! -t 0 ]; then
	echo "an interactive terminal is required" >&2
	exit 2
fi
if ! command -v go >/dev/null 2>&1; then
	echo "go is required" >&2
	exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required" >&2
	exit 1
fi

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
management_port=${APP_SERVER_MANAGEMENT_PORT:-9090}
management_url=${ADMIN_INIT_MANAGEMENT_URL:-http://127.0.0.1:${management_port}}
timeout_seconds=${ADMIN_INIT_TIMEOUT_SECONDS:-60}
case "$timeout_seconds" in
	""|*[!0-9]*)
		echo "ADMIN_INIT_TIMEOUT_SECONDS must be a positive integer" >&2
		exit 2
		;;
esac
if [ "$timeout_seconds" -le 0 ]; then
	echo "ADMIN_INIT_TIMEOUT_SECONDS must be a positive integer" >&2
	exit 2
fi

if curl --fail --silent --max-time 2 "$management_url/livez" >/dev/null 2>&1; then
	echo "an API management server is already running at $management_url; stop it before initialization" >&2
	exit 1
fi

printf 'Administrator username [admin]: '
IFS= read -r admin_username
admin_username=${admin_username:-admin}

temp_dir=""
api_pid=""
cleanup() {
	if [ -n "$api_pid" ] && kill -0 "$api_pid" >/dev/null 2>&1; then
		kill -INT "$api_pid" >/dev/null 2>&1 || true
		wait "$api_pid" >/dev/null 2>&1 || true
	fi
	if [ -n "$temp_dir" ]; then
		[ ! -f "$temp_dir/api" ] || unlink "$temp_dir/api"
		[ ! -f "$temp_dir/api.log" ] || unlink "$temp_dir/api.log"
		rmdir "$temp_dir" >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

printf 'Administrator password (at least 8 characters): '
IFS= read -r admin_password

printf 'Confirm administrator password: '
IFS= read -r admin_password_confirmation

if [ "$admin_password" != "$admin_password_confirmation" ]; then
	echo "passwords do not match" >&2
	exit 2
fi
if [ "${#admin_password}" -lt 8 ]; then
	echo "password must contain at least 8 characters" >&2
	exit 2
fi
unset admin_password_confirmation

temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/go-service-main-init-admin.XXXXXX")
api_binary="$temp_dir/api"
api_log="$temp_dir/api.log"

cd "$project_dir"
echo "building temporary API..."
go build -trimpath -o "$api_binary" ./cmd/api

APP_IDENTITY_BOOTSTRAP_USER="$admin_username" \
	APP_IDENTITY_BOOTSTRAP_PASSWORD="$admin_password" \
	"$api_binary" >"$api_log" 2>&1 &
api_pid=$!
unset admin_password

elapsed=0
ready=0
while [ "$elapsed" -lt "$timeout_seconds" ]; do
	if ! kill -0 "$api_pid" >/dev/null 2>&1; then
		wait "$api_pid" >/dev/null 2>&1 || true
		api_pid=""
		echo "temporary API exited before becoming ready" >&2
		tail -n 40 "$api_log" >&2
		exit 1
	fi
	if curl --fail --silent --max-time 2 "$management_url/readyz" >/dev/null 2>&1; then
		ready=1
		break
	fi
	sleep 1
	elapsed=$((elapsed + 1))
done

if [ "$ready" -ne 1 ]; then
	echo "temporary API did not become ready within ${timeout_seconds}s" >&2
	tail -n 40 "$api_log" >&2
	exit 1
fi

if grep -q "bootstrap administrator created" "$api_log"; then
	result=created
elif grep -q "bootstrap administrator skipped" "$api_log"; then
	result=skipped
else
	result=unknown
fi

kill -INT "$api_pid" >/dev/null 2>&1 || true
wait "$api_pid" >/dev/null 2>&1 || true
api_pid=""

case "$result" in
	created)
		echo "ROOT administrator '$admin_username' created successfully; the temporary API has been stopped"
		;;
	skipped)
		echo "administrator was not created because the database already has an active user" >&2
		exit 1
		;;
	*)
		echo "temporary API became ready, but the bootstrap result could not be confirmed" >&2
		tail -n 40 "$api_log" >&2
		exit 1
		;;
esac
