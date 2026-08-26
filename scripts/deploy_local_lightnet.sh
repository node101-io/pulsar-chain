#!/usr/bin/env bash

# Recreates one local Mina/Pulsar development stack. Only the selected Pulsar
# Compose project's owned state is removed; a running owned Lightnet is reused.

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

VALIDATOR_COUNT=""
PROJECT_NAME=""
STATE_ROOT="${PULSAR_DOCKER_STATE_ROOT:-$REPO_ROOT/.docker}"
REUSE_LIGHTNET=0

LIGHTNET_CONTAINER="${LIGHTNET_CONTAINER:-mina-local-lightnet}"
DEFAULT_LIGHTNET_IMAGE="o1labs/mina-local-network@sha256:33e349241f5f3e8d336e5de9b35de2d4339fd8713b309e2b1b5fc375c2605b58"
LIGHTNET_IMAGE="${LIGHTNET_IMAGE:-$DEFAULT_LIGHTNET_IMAGE}"
LIGHTNET_POSTGRES_PORT="${LIGHTNET_POSTGRES_PORT:-15432}"
LIGHTNET_READY_HEIGHT="${LIGHTNET_READY_HEIGHT:-4}"
LIGHTNET_OWNERSHIP_LABEL="io.node101.pulsar.local-testnet"
LIGHTNET_OWNERSHIP_VALUE="mina-lightnet"

WRAPPER_SOURCE="${ARCHIVE_WRAPPER_SOURCE:-$REPO_ROOT/../archive-wrapper}"
WRAPPER_SHA="${ARCHIVE_WRAPPER_SHA:-cd42a203ac6b43d24d9fbd57c323ecd52ea52bd5}"
WRAPPER_IMAGE="${ARCHIVE_WRAPPER_IMAGE:-archive-wrapper:lightnet}"
PULSAR_IMAGE=""
BRIDGE_CONFIRMATION_DEPTH="${BRIDGE_CONFIRMATION_DEPTH:-3}"
BRIDGE_START_BLOCK_HEIGHT="${BRIDGE_START_BLOCK_HEIGHT:-1}"
BRIDGE_MAX_BLOCK_RANGE="${BRIDGE_MAX_BLOCK_RANGE:-1000}"
VALIDATOR_STARTUP_TIMEOUT="${VALIDATOR_STARTUP_TIMEOUT:-600}"

case "$(uname -m)" in
  arm64 | aarch64) DEFAULT_DOCKER_PLATFORM="linux/arm64" ;;
  *) DEFAULT_DOCKER_PLATFORM="linux/amd64" ;;
esac
DOCKER_PLATFORM="${DOCKER_PLATFORM:-$DEFAULT_DOCKER_PLATFORM}"

usage() {
  cat <<'EOF'
usage: scripts/deploy_local_lightnet.sh <validator-count>

Recreates the local development stack with:
  - an existing running Mina Lightnet, or a new Dockerized Lightnet
  - the blocks_inserted LISTEN/NOTIFY trigger
  - one shared archive-wrapper in Docker
  - the requested number of Pulsar validators in Docker

Destructive behavior:
  The selected Compose project's validator and archive-wrapper volumes are
  removed before redeployment, but only after its ownership marker is verified.
  A running owned Lightnet is reused. A stopped owned Lightnet is replaced.
  Unrelated Docker resources, images, and host-side ~/.pulsar* directories are
  left untouched.

Optional environment variables:
  LIGHTNET_CONTAINER         container name (default: mina-local-lightnet)
  LIGHTNET_IMAGE             image override (default pinned below)
  LIGHTNET_POSTGRES_PORT     host PostgreSQL port (default: 15432)
  LIGHTNET_READY_HEIGHT      minimum archive height (default: 4)
  ARCHIVE_WRAPPER_SOURCE     checkout path (default: ../archive-wrapper)
  ARCHIVE_WRAPPER_SHA        commit to build (default pinned in the script)
  ARCHIVE_WRAPPER_IMAGE      built image name (default: archive-wrapper:lightnet)
  PULSAR_DOCKER_PROJECT      Compose project (default: pulsar-testnet-N)
  PULSAR_DOCKER_STATE_ROOT   generated state root (default: .docker)
  PULSAR_DOCKER_IMAGE        Pulsar image name (default derived from N)
  DOCKER_PLATFORM            linux/arm64 or linux/amd64 (default: host arch)
  VALIDATOR_STARTUP_TIMEOUT  Compose wait timeout in seconds (default: 600)
  BRIDGE_CONFIRMATION_DEPTH  local bridge confirmation depth (default: 3)
  BRIDGE_START_BLOCK_HEIGHT  local bridge start height (default: 1)
  BRIDGE_MAX_BLOCK_RANGE     local bridge query range (default: 1000)
  SMART_ACCOUNTS_VERIFICATION_KEY_HASH
                             32-byte smartaccounts verification-key hash in hex
                             or base64 (default: value from config.yml)
  SMART_ACCOUNT_NOIR_FIXTURE_DIR
                             real smart-account Noir fixture directory; when
                             set, verification_key_hash is derived from its vk
  PULSAR_VERIFIER_IMAGE      production verifier image; required with the
                             smart-account Noir fixture

Default Mina Lightnet image:
  o1labs/mina-local-network@sha256:33e349241f5f3e8d336e5de9b35de2d4339fd8713b309e2b1b5fc375c2605b58

Full prerequisites, lifecycle, cleanup, and reset documentation:
  docs/local-lightnet-deployment.md
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

validate_positive_int() {
  local name="$1"
  local value="$2"

  if ! [[ "$value" =~ ^[0-9]+$ ]] || (( value < 1 )); then
    echo "$name must be a positive integer, got: $value" >&2
    exit 1
  fi
}

validate_port() {
  local name="$1"
  local value="$2"

  validate_positive_int "$name" "$value"
  if (( value > 65535 )); then
    echo "$name must not exceed 65535, got: $value" >&2
    exit 1
  fi
}

install_notification_trigger() {
  docker exec -i "$LIGHTNET_CONTAINER" \
    psql -v ON_ERROR_STOP=1 -U postgres -d archive <<'SQL'
CREATE OR REPLACE FUNCTION public.archive_wrapper_notify_block_inserted()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify(
    'blocks_inserted',
    json_build_object('height', NEW.height)::text
  );
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS archive_wrapper_blocks_inserted_notify ON public.blocks;

CREATE TRIGGER archive_wrapper_blocks_inserted_notify
AFTER INSERT ON public.blocks
FOR EACH ROW
EXECUTE FUNCTION public.archive_wrapper_notify_block_inserted();
SQL
}

cleanup_selected_project() {
  local compose_file
  local generated_root
  local marker
  local marker_content

  generated_root="$STATE_ROOT/$PROJECT_NAME"
  compose_file="$generated_root/compose.json"
  marker="$generated_root/.pulsar-docker-project"

  if [[ ! -e "$generated_root" && ! -L "$generated_root" ]]; then
    echo "    no previous state for Compose project: $PROJECT_NAME"
    return
  fi

  if [[ ! -d "$generated_root" || -L "$generated_root" ]]; then
    echo "refusing unsafe generated project directory: $generated_root" >&2
    exit 1
  fi

  if [[ ! -f "$marker" || -L "$marker" ]]; then
    echo "refusing to remove unowned generated directory: $generated_root" >&2
    exit 1
  fi

  marker_content="$(cat -- "$marker")"
  if [[ "$marker_content" != "pulsar-docker-project:$PROJECT_NAME" ]]; then
    echo "refusing to remove generated directory with invalid marker: $generated_root" >&2
    exit 1
  fi

  if [[ ! -f "$compose_file" || -L "$compose_file" ]]; then
    echo "refusing to remove project without an owned Compose file: $compose_file" >&2
    exit 1
  fi

  echo "    stopping Compose project: $PROJECT_NAME"
  if ! docker compose \
    --project-name "$PROJECT_NAME" \
    -f "$compose_file" \
    down --volumes --remove-orphans; then
    echo "failed to clean Compose project: $PROJECT_NAME" >&2
    echo "preserving project metadata at: $generated_root" >&2
    exit 1
  fi

  rm -rf -- "$generated_root"
}

lightnet_container_exists() {
  local inspected_name

  inspected_name="$(docker inspect -f '{{.Name}}' "$LIGHTNET_CONTAINER" 2>/dev/null || true)"
  [[ "$inspected_name" == "/$LIGHTNET_CONTAINER" ]]
}

lightnet_container_is_owned() {
  local container_ref="${1:-$LIGHTNET_CONTAINER}"
  local ownership

  ownership="$(docker inspect \
    -f "{{ index .Config.Labels \"$LIGHTNET_OWNERSHIP_LABEL\" }}" \
    "$container_ref" 2>/dev/null || true)"
  [[ "$ownership" == "$LIGHTNET_OWNERSHIP_VALUE" ]]
}

cleanup_owned_stale_lightnet() {
  local container_id
  local inspected_name

  if (( REUSE_LIGHTNET == 1 )) || ! lightnet_container_exists; then
    return
  fi

  container_id="$(docker inspect -f '{{.Id}}' "$LIGHTNET_CONTAINER")"
  inspected_name="$(docker inspect -f '{{.Name}}' "$container_id")"
  if [[ "$inspected_name" != "/$LIGHTNET_CONTAINER" ]] || \
    ! lightnet_container_is_owned "$container_id"; then
    echo "refusing to remove unowned container named $LIGHTNET_CONTAINER" >&2
    exit 1
  fi

  echo "    removing stopped owned Lightnet container: $LIGHTNET_CONTAINER"
  docker rm -fv "$container_id"
}

wait_for_lightnet_archive() {
  local archive_height=0
  local attempt
  local blocks_table

  echo "    waiting for archive schema and Mina height $LIGHTNET_READY_HEIGHT"
  for ((attempt = 1; attempt <= 300; attempt++)); do
    if docker exec "$LIGHTNET_CONTAINER" \
      pg_isready -U postgres -d archive >/dev/null 2>&1; then
      blocks_table="$({
        docker exec "$LIGHTNET_CONTAINER" \
          psql -U postgres -d archive -Atqc \
          "SELECT to_regclass('public.blocks') IS NOT NULL;"
      } 2>/dev/null || true)"

      if [[ "$blocks_table" == "t" ]]; then
        archive_height="$({
          docker exec "$LIGHTNET_CONTAINER" \
            psql -U postgres -d archive -Atqc \
            "SELECT COALESCE(MAX(height), 0) FROM public.blocks;"
        } 2>/dev/null || printf '0')"

        if [[ "$archive_height" =~ ^[0-9]+$ ]] && \
          (( archive_height >= LIGHTNET_READY_HEIGHT )); then
          echo "    Lightnet archive height: $archive_height"
          return
        fi
      fi
    fi

    sleep 2
  done

  echo "Lightnet archive did not become ready in time" >&2
  docker logs --tail 150 "$LIGHTNET_CONTAINER" >&2 || true
  exit 1
}

detect_running_lightnet() {
  if ! lightnet_container_exists; then
    return
  fi

  if ! lightnet_container_is_owned; then
    echo "container name $LIGHTNET_CONTAINER is already used by an unowned container" >&2
    echo "choose another LIGHTNET_CONTAINER or remove it explicitly" >&2
    exit 1
  fi

  if [[ "$(docker inspect -f '{{.State.Running}}' "$LIGHTNET_CONTAINER")" == "true" ]]; then
    REUSE_LIGHTNET=1
  fi
}

case "${1:-}" in
  -h | --help)
    usage
    exit 0
    ;;
esac

if (( $# != 1 )); then
  usage >&2
  exit 2
fi

VALIDATOR_COUNT="$1"
validate_positive_int "validator count" "$VALIDATOR_COUNT"
validate_port "last validator RPC port" "$((26657 + ((VALIDATOR_COUNT - 1) * 10)))"
validate_port "last validator REST port" "$((1317 + VALIDATOR_COUNT - 1))"

PROJECT_NAME="${PULSAR_DOCKER_PROJECT:-pulsar-testnet-${VALIDATOR_COUNT}}"
PULSAR_IMAGE="${PULSAR_DOCKER_IMAGE:-pulsar-chain:local-${VALIDATOR_COUNT}-validators}"

if ! [[ "$PROJECT_NAME" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ ]]; then
  echo "invalid Docker Compose project name: $PROJECT_NAME" >&2
  exit 1
fi

if [[ -z "${HOME:-}" || "$HOME" == "/" ]]; then
  echo "refusing unsafe HOME value: ${HOME:-<empty>}" >&2
  exit 1
fi

validate_port "Lightnet PostgreSQL port" "$LIGHTNET_POSTGRES_PORT"
validate_positive_int "Lightnet ready height" "$LIGHTNET_READY_HEIGHT"
validate_positive_int "validator startup timeout" "$VALIDATOR_STARTUP_TIMEOUT"
validate_positive_int "bridge confirmation depth" "$BRIDGE_CONFIRMATION_DEPTH"
validate_positive_int "bridge start block height" "$BRIDGE_START_BLOCK_HEIGHT"
validate_positive_int "bridge max block range" "$BRIDGE_MAX_BLOCK_RANGE"

case "$DOCKER_PLATFORM" in
  linux/arm64 | linux/amd64) ;;
  *)
    echo "Docker platform must be linux/arm64 or linux/amd64, got: $DOCKER_PLATFORM" >&2
    exit 1
    ;;
esac

require_cmd awk
require_cmd curl
require_cmd docker
require_cmd git
require_cmd python3

if [[ -n "${SMART_ACCOUNT_NOIR_FIXTURE_DIR:-}" ]]; then
  if [[ ! -f "$SMART_ACCOUNT_NOIR_FIXTURE_DIR/vk" ]]; then
    echo "smart-account Noir verification key not found: $SMART_ACCOUNT_NOIR_FIXTURE_DIR/vk" >&2
    exit 1
  fi
  if [[ -z "${PULSAR_VERIFIER_IMAGE:-}" ]]; then
    echo "PULSAR_VERIFIER_IMAGE is required with SMART_ACCOUNT_NOIR_FIXTURE_DIR" >&2
    exit 1
  fi

  fixture_verification_key_hash="$(python3 -c 'import hashlib,pathlib,sys; print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())' "$SMART_ACCOUNT_NOIR_FIXTURE_DIR/vk")"
  fixture_verification_key_hash_base64="$(python3 -c 'import base64,sys; print(base64.b64encode(bytes.fromhex(sys.argv[1])).decode())' "$fixture_verification_key_hash")"
  if [[ -n "${SMART_ACCOUNTS_VERIFICATION_KEY_HASH:-}" &&
    "$SMART_ACCOUNTS_VERIFICATION_KEY_HASH" != "$fixture_verification_key_hash" &&
    "$SMART_ACCOUNTS_VERIFICATION_KEY_HASH" != "$fixture_verification_key_hash_base64" ]]; then
    echo "SMART_ACCOUNTS_VERIFICATION_KEY_HASH does not match SHA-256($SMART_ACCOUNT_NOIR_FIXTURE_DIR/vk)" >&2
    exit 1
  fi
  export SMART_ACCOUNTS_VERIFICATION_KEY_HASH="$fixture_verification_key_hash"
fi

docker compose version >/dev/null
docker buildx version >/dev/null

STATE_ROOT="$(python3 -c 'import os, sys; print(os.path.realpath(sys.argv[1]))' "$STATE_ROOT")"
if [[ "$STATE_ROOT" == "/" || "$STATE_ROOT" == "$REPO_ROOT" || "$STATE_ROOT" == "$HOME" ]]; then
  echo "refusing unsafe Docker state root: $STATE_ROOT" >&2
  exit 1
fi

if [[ ! -x "$SCRIPT_DIR/docker_testnet.sh" ]]; then
  echo "Docker testnet script not found at: $SCRIPT_DIR/docker_testnet.sh" >&2
  exit 1
fi

if [[ ! -d "$WRAPPER_SOURCE/.git" ]]; then
  echo "archive-wrapper checkout not found at: $WRAPPER_SOURCE" >&2
  exit 1
fi
WRAPPER_SOURCE="$(cd -- "$WRAPPER_SOURCE" && pwd)"
git -C "$WRAPPER_SOURCE" cat-file -e "${WRAPPER_SHA}^{commit}"

detect_running_lightnet

echo "==> 1/5 Removing previous Pulsar and archive-wrapper state"
if (( REUSE_LIGHTNET == 1 )); then
  echo "    preserving running owned Lightnet container: $LIGHTNET_CONTAINER"
else
  echo "    no running owned Lightnet found"
fi
cleanup_selected_project
cleanup_owned_stale_lightnet

if (( REUSE_LIGHTNET == 1 )); then
  echo "==> 2/5 Reusing running Mina Lightnet with archive PostgreSQL"
else
  echo "==> 2/5 Starting Mina Lightnet with archive PostgreSQL"
  docker run -d \
    --name "$LIGHTNET_CONTAINER" \
    --label "${LIGHTNET_OWNERSHIP_LABEL}=${LIGHTNET_OWNERSHIP_VALUE}" \
    -p 127.0.0.1:3085:3085 \
    -p 127.0.0.1:8080:8080 \
    -p 127.0.0.1:8181:8181 \
    -p 127.0.0.1:8282:8282 \
    -p "127.0.0.1:${LIGHTNET_POSTGRES_PORT}:5432" \
    -e NETWORK_TYPE=single-node \
    -e PROOF_LEVEL=none \
    -e RUN_ARCHIVE_NODE=true \
    -e LOG_LEVEL=Info \
    -e SLOT_TIME=20000 \
    "$LIGHTNET_IMAGE" >/dev/null
fi

wait_for_lightnet_archive

echo "==> 3/5 Installing the blocks_inserted LISTEN/NOTIFY trigger"
install_notification_trigger

trigger_exists="$(docker exec "$LIGHTNET_CONTAINER" \
  psql -U postgres -d archive -Atqc \
  "SELECT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'archive_wrapper_blocks_inserted_notify');")"
if [[ "$trigger_exists" != "t" ]]; then
  echo "Lightnet notification trigger was not installed" >&2
  exit 1
fi

echo "==> 4/5 Building the shared archive-wrapper image"
git -C "$WRAPPER_SOURCE" archive "$WRAPPER_SHA" |
  docker buildx build \
    --load \
    --platform "$DOCKER_PLATFORM" \
    --build-arg COMMIT_SHA="$WRAPPER_SHA" \
    -t "$WRAPPER_IMAGE" \
    -

PG_USER="$(docker exec "$LIGHTNET_CONTAINER" printenv POSTGRES_USER)"
PG_PASSWORD="$(docker exec "$LIGHTNET_CONTAINER" printenv POSTGRES_PASSWORD)"
PG_DB="$(docker exec "$LIGHTNET_CONTAINER" printenv POSTGRES_DB)"
MAPPED_PG_PORT="$(docker port "$LIGHTNET_CONTAINER" 5432/tcp | awk -F: 'NR == 1 {print $NF}')"

if [[ -z "$PG_USER" || -z "$PG_PASSWORD" || -z "$PG_DB" || -z "$MAPPED_PG_PORT" ]]; then
  echo "could not read Lightnet PostgreSQL connection details" >&2
  exit 1
fi

# Idempotently reinstall immediately before wrapper startup. This closes the
# race where a late Lightnet schema initialization could remove the trigger.
install_notification_trigger >/dev/null

export ARCHIVE_WRAPPER_MODE="shared"
export ARCHIVE_WRAPPER_IMAGE="$WRAPPER_IMAGE"
export ARCHIVE_WRAPPER_ADD_HOST_GATEWAY=1
export PULSAR_DOCKER_IMAGE="$PULSAR_IMAGE"
export PULSAR_DOCKER_PROJECT="$PROJECT_NAME"
export PULSAR_DOCKER_STATE_ROOT="$STATE_ROOT"
export PULSAR_BIND_HOST="127.0.0.1"
export POSTGRES_URI="postgres://${PG_USER}:${PG_PASSWORD}@host.docker.internal:${MAPPED_PG_PORT}/${PG_DB}?sslmode=disable"
export BRIDGE_CONFIRMATION_DEPTH
export BRIDGE_START_BLOCK_HEIGHT
export BRIDGE_MAX_BLOCK_RANGE
export VALIDATOR_STARTUP_TIMEOUT

echo "==> 5/5 Starting Pulsar with $VALIDATOR_COUNT validators"
"$SCRIPT_DIR/docker_testnet.sh" up "$VALIDATOR_COUNT"

for ((index = 1; index <= VALIDATOR_COUNT; index++)); do
  rpc_port="$((26657 + ((index - 1) * 10)))"
  api_port="$((1317 + index - 1))"

  curl -fsS "http://127.0.0.1:${rpc_port}/status" >/dev/null
  curl -fsS \
    "http://127.0.0.1:${api_port}/cosmos/base/tendermint/v1beta1/node_info" \
    >/dev/null
done

echo
echo "Deployment is healthy."
for ((index = 1; index <= VALIDATOR_COUNT; index++)); do
  rpc_port="$((26657 + ((index - 1) * 10)))"
  api_port="$((1317 + index - 1))"
  echo "  Validator $index REST: http://localhost:$api_port"
  echo "  Validator $index RPC:  http://localhost:$rpc_port"
done
echo "  Lightnet GraphQL: http://localhost:8080/graphql"
