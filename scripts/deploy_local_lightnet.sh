#!/usr/bin/env bash

# Recreates the local Mina/Pulsar development stack. Existing Pulsar and
# archive-wrapper state is removed, while a running Mina Lightnet is reused.

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

VALIDATOR_COUNT=""
PROJECT_NAME=""
STATE_ROOT="${PULSAR_DOCKER_STATE_ROOT:-$REPO_ROOT/.docker}"
REUSE_LIGHTNET=0

LIGHTNET_CONTAINER="${LIGHTNET_CONTAINER:-mina-local-lightnet}"
LIGHTNET_IMAGE="${LIGHTNET_IMAGE:-o1labs/mina-local-network:compatible-latest-lightnet}"
LIGHTNET_POSTGRES_PORT="${LIGHTNET_POSTGRES_PORT:-15432}"
LIGHTNET_READY_HEIGHT="${LIGHTNET_READY_HEIGHT:-4}"

WRAPPER_SOURCE="${ARCHIVE_WRAPPER_SOURCE:-$REPO_ROOT/../archive-wrapper}"
WRAPPER_SHA="${ARCHIVE_WRAPPER_SHA:-cd42a203ac6b43d24d9fbd57c323ecd52ea52bd5}"
WRAPPER_IMAGE="${ARCHIVE_WRAPPER_IMAGE:-archive-wrapper:lightnet}"
PULSAR_IMAGE=""

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

Optional environment variables:
  ARCHIVE_WRAPPER_SOURCE   archive-wrapper checkout (default: ../archive-wrapper)
  ARCHIVE_WRAPPER_SHA      archive-wrapper commit to build
  DOCKER_PLATFORM          linux/arm64 or linux/amd64
  LIGHTNET_POSTGRES_PORT   host PostgreSQL port (default: 15432)
  PULSAR_DOCKER_PROJECT    Compose project name (default: pulsar-testnet-N)
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

is_related_container() {
  local image="$1"
  local name="$2"

  if [[ "$name" == "$PROJECT_NAME"-* ]]; then
    return 0
  fi

  case "$name" in
    pulsar-testnet-* | pulsar-wrapper-* | archive-wrapper* | mina-local-lightnet*)
      return 0
      ;;
  esac

  case "$image" in
    pulsar-chain:* | pulsar-chain@* | */pulsar-chain:* | */pulsar-chain@* | \
      archive-wrapper:* | archive-wrapper@* | */archive-wrapper:* | */archive-wrapper@* | \
      o1labs/mina-local-network:* | o1labs/mina-local-network@*)
      return 0
      ;;
  esac

  return 1
}

is_related_resource_name() {
  if [[ "$1" == "${PROJECT_NAME}_"* || "$1" == "$PROJECT_NAME"-* ]]; then
    return 0
  fi

  case "$1" in
    pulsar-testnet-* | pulsar-wrapper-* | archive-wrapper* | mina-local-lightnet*)
      return 0
      ;;
    *) return 1 ;;
  esac
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

cleanup_generated_projects() {
  local compose_file
  local generated_root
  local marker
  local marker_content
  local project

  if [[ ! -d "$STATE_ROOT" ]]; then
    return
  fi

  while IFS= read -r compose_file; do
    generated_root="$(dirname -- "$compose_file")"
    project="$(basename -- "$generated_root")"
    marker="$generated_root/.pulsar-docker-project"

    if [[ "$project" != pulsar-testnet-* && "$project" != "$PROJECT_NAME" ]]; then
      continue
    fi

    case "$generated_root/" in
      "$STATE_ROOT/"*) ;;
      *)
        echo "refusing generated project path outside state root: $generated_root" >&2
        exit 1
        ;;
    esac

    if [[ ! -f "$marker" || -L "$marker" ]]; then
      echo "skipping unowned generated directory: $generated_root" >&2
      continue
    fi

    marker_content="$(cat -- "$marker")"
    if [[ "$marker_content" != "pulsar-docker-project:$project" ]]; then
      echo "skipping generated directory with invalid marker: $generated_root" >&2
      continue
    fi

    echo "    stopping Compose project: $project"
    docker compose \
      --project-name "$project" \
      -f "$compose_file" \
      down --volumes --remove-orphans || true

    rm -rf -- "$generated_root"
  done < <(find "$STATE_ROOT" -mindepth 2 -maxdepth 2 -type f -name compose.json -print)
}

cleanup_related_containers() {
  local container_id
  local container_image
  local container_name
  local -a ids=()
  local -a names=()

  while IFS=$'\t' read -r container_id container_image container_name; do
    if (( REUSE_LIGHTNET == 1 )) && [[ "$container_name" == "$LIGHTNET_CONTAINER" ]]; then
      continue
    fi

    if is_related_container "$container_image" "$container_name"; then
      ids+=("$container_id")
      names+=("$container_name")
    fi
  done < <(docker ps -a --format '{{.ID}}\t{{.Image}}\t{{.Names}}')

  if (( ${#ids[@]} == 0 )); then
    echo "    no related containers found"
    return
  fi

  echo "    removing containers: ${names[*]}"
  docker rm -fv "${ids[@]}"
}

cleanup_related_volumes() {
  local name
  local -a volumes=()

  while IFS= read -r name; do
    if (( REUSE_LIGHTNET == 1 )) && [[ "$name" == mina-local-lightnet* ]]; then
      continue
    fi

    if is_related_resource_name "$name"; then
      volumes+=("$name")
    fi
  done < <(docker volume ls --format '{{.Name}}')

  if (( ${#volumes[@]} == 0 )); then
    echo "    no related named volumes found"
    return
  fi

  echo "    removing volumes: ${volumes[*]}"
  docker volume rm -f "${volumes[@]}"
}

cleanup_related_networks() {
  local name
  local -a networks=()

  while IFS= read -r name; do
    if (( REUSE_LIGHTNET == 1 )) && [[ "$name" == mina-local-lightnet* ]]; then
      continue
    fi

    if is_related_resource_name "$name"; then
      networks+=("$name")
    fi
  done < <(docker network ls --format '{{.Name}}')

  if (( ${#networks[@]} == 0 )); then
    echo "    no related networks found"
    return
  fi

  echo "    removing networks: ${networks[*]}"
  docker network rm "${networks[@]}" || true
}

cleanup_related_images() {
  local basename
  local repository
  local tag
  local -a images=()

  while IFS=$'\t' read -r repository tag; do
    basename="${repository##*/}"
    case "$basename" in
      pulsar-chain | archive-wrapper | mina-local-network)
        if (( REUSE_LIGHTNET == 1 )) && [[ "$basename" == "mina-local-network" ]]; then
          continue
        fi

        if [[ "$tag" != "<none>" ]]; then
          images+=("${repository}:${tag}")
        fi
        ;;
    esac
  done < <(docker image ls --format '{{.Repository}}\t{{.Tag}}')

  if (( ${#images[@]} == 0 )); then
    echo "    no related images found"
    return
  fi

  echo "    removing images: ${images[*]}"
  docker image rm -f "${images[@]}"
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
  local container_image
  local container_name
  local -a candidates=()

  if [[ "$(docker inspect -f '{{.State.Running}}' "$LIGHTNET_CONTAINER" 2>/dev/null || true)" == "true" ]]; then
    REUSE_LIGHTNET=1
    return
  fi

  while IFS=$'\t' read -r container_image container_name; do
    case "$container_image" in
      o1labs/mina-local-network:* | o1labs/mina-local-network@*)
        candidates+=("$container_name")
        ;;
    esac
  done < <(docker ps --format '{{.Image}}\t{{.Names}}')

  case "${#candidates[@]}" in
    0) return ;;
    1)
      LIGHTNET_CONTAINER="${candidates[0]}"
      REUSE_LIGHTNET=1
      ;;
    *)
      echo "multiple running Mina Lightnet containers found: ${candidates[*]}" >&2
      echo "set LIGHTNET_CONTAINER to select one" >&2
      exit 1
      ;;
  esac
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

require_cmd awk
require_cmd curl
require_cmd docker
require_cmd find
require_cmd git
require_cmd python3

docker compose version >/dev/null
docker buildx version >/dev/null

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
  echo "    preserving running Lightnet container: $LIGHTNET_CONTAINER"
else
  echo "    no running Lightnet found; stale Lightnet resources will be removed"
fi
cleanup_generated_projects
cleanup_related_containers
cleanup_related_volumes
cleanup_related_networks
cleanup_related_images

rm -rf -- "$HOME/.pulsar"
while IFS= read -r node_home; do
  node_home_name="$(basename -- "$node_home")"
  if ! [[ "$node_home_name" =~ ^\.pulsar-node[0-9]+$ ]]; then
    continue
  fi
  rm -rf -- "$node_home"
done < <(find "$HOME" -mindepth 1 -maxdepth 1 -type d -name '.pulsar-node[0-9]*' -print)

if (( REUSE_LIGHTNET == 1 )); then
  echo "==> 2/5 Reusing running Mina Lightnet with archive PostgreSQL"
else
  echo "==> 2/5 Starting Mina Lightnet with archive PostgreSQL"
  docker run -d \
    --name "$LIGHTNET_CONTAINER" \
    --label io.node101.pulsar.local-testnet=mina-lightnet \
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
export PULSAR_DOCKER_IMAGE="$PULSAR_IMAGE"
export PULSAR_DOCKER_PROJECT="$PROJECT_NAME"
export PULSAR_DOCKER_STATE_ROOT="$STATE_ROOT"
export PULSAR_BIND_HOST="127.0.0.1"
export POSTGRES_URI="postgres://${PG_USER}:${PG_PASSWORD}@host.docker.internal:${MAPPED_PG_PORT}/${PG_DB}?sslmode=disable"
export BRIDGE_CONFIRMATION_DEPTH="${BRIDGE_CONFIRMATION_DEPTH:-3}"
export BRIDGE_START_BLOCK_HEIGHT="${BRIDGE_START_BLOCK_HEIGHT:-1}"
export BRIDGE_MAX_BLOCK_RANGE="${BRIDGE_MAX_BLOCK_RANGE:-1000}"
export VALIDATOR_STARTUP_TIMEOUT="${VALIDATOR_STARTUP_TIMEOUT:-600}"

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
