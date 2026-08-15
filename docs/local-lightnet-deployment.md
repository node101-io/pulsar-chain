# Local Lightnet deployment

`scripts/deploy_local_lightnet.sh` recreates a local development environment
containing Mina Lightnet, the PostgreSQL notification trigger required by
archive-wrapper, one shared archive-wrapper, and a requested number of Pulsar
validators. It is intended for local development and manual testing, not for a
production deployment.

Run it from the repository root with a positive validator count:

```bash
./scripts/deploy_local_lightnet.sh 3
```

## Prerequisites

The host must provide:

- Bash, `awk`, `curl`, Git, and Python 3.
- Docker Engine or Docker Desktop with the Compose and Buildx plugins.
- Docker support for the `host-gateway` value in Compose `extra_hosts`.
- An archive-wrapper Git checkout. It defaults to the sibling directory
  `../archive-wrapper` and must contain the configured commit.
- Free loopback ports for Mina Lightnet and all requested Pulsar validators.

On Linux, generated archive-wrapper services map
`host.docker.internal` to `host-gateway`. This lets the wrapper reach the
Lightnet PostgreSQL port published on the host; Docker Desktop provides the same
hostname on macOS and Windows.

The default Lightnet image is immutable and is the image used during manual
deployment verification for this script:

```text
o1labs/mina-local-network@sha256:33e349241f5f3e8d336e5de9b35de2d4339fd8713b309e2b1b5fc375c2605b58
```

Setting `LIGHTNET_IMAGE` allows intentional testing with another version, but a
mutable tag makes that run less reproducible.

## Deployment flow

The script performs five stages:

1. Validate inputs and remove only the selected, marker-owned Pulsar Compose
   project and its named volumes.
2. Reuse a running owned Mina Lightnet or start the pinned Lightnet image.
3. Wait for the archive schema and configured height, then install the
   `blocks_inserted` PostgreSQL `LISTEN`/`NOTIFY` trigger.
4. Build the shared archive-wrapper image from the configured Git commit.
5. Generate the Compose project, start the wrapper and validators, and verify
   every validator's REST and CometBFT RPC endpoint.

REST and RPC ports are bound to `127.0.0.1`. Validator `N` uses REST port
`1317 + N - 1` and RPC port `26657 + (N - 1) * 10`. The Lightnet PostgreSQL
host port defaults to `15432`, and its GraphQL endpoint is available at
`http://localhost:8080/graphql`.

## Reuse and cleanup contract

The Compose project defaults to `pulsar-testnet-<validator-count>`, with
generated files under `.docker/<project-name>`. Before deleting anything, the
script requires all of the following to match:

- The exact `PULSAR_DOCKER_PROJECT` directory under
  `PULSAR_DOCKER_STATE_ROOT`.
- A regular `.pulsar-docker-project` ownership marker containing the exact
  project name.
- A regular generated `compose.json` file.

After validation, `docker compose down --volumes --remove-orphans` removes the
selected project's containers, validator homes, archive-wrapper LevelDB volume,
and Compose network. The generated directory is deleted only if Compose cleanup
succeeds. If Docker is unavailable or cleanup fails, the script stops and keeps
the Compose file and ownership marker for recovery.

The script does not scan for or remove other Docker projects, containers,
volumes, networks, or images. It also does not remove `~/.pulsar` or
`~/.pulsar-node*` directories.

Lightnet is a standalone container outside the generated Compose project. Its
behavior depends on the exact `LIGHTNET_CONTAINER` name and the ownership label
`io.node101.pulsar.local-testnet=mina-lightnet`:

| Lightnet state | Deployment behavior |
| --- | --- |
| No exact-name container | Start the pinned image with the ownership label. |
| Running and owned | Reuse the container and its existing archive database. |
| Stopped and owned | Remove it with its anonymous volumes, then create a new one. |
| Exact name without the ownership label | Stop with an error and do not mutate it. |
| Different container name | Leave it untouched. |

The notification trigger is installed idempotently on both first run and reuse.

## Supported overrides

### Lightnet and Docker

| Variable | Default | Effect |
| --- | --- | --- |
| `LIGHTNET_CONTAINER` | `mina-local-lightnet` | Exact standalone Lightnet container name. |
| `LIGHTNET_IMAGE` | Pinned digest above | Image used only when creating Lightnet. |
| `LIGHTNET_POSTGRES_PORT` | `15432` | Host port mapped to Lightnet PostgreSQL `5432`. |
| `LIGHTNET_READY_HEIGHT` | `4` | Minimum archived Mina height before continuing. |
| `DOCKER_PLATFORM` | Host architecture | Buildx target, normally `linux/arm64` or `linux/amd64`. |

### Archive-wrapper

| Variable | Default | Effect |
| --- | --- | --- |
| `ARCHIVE_WRAPPER_SOURCE` | `../archive-wrapper` | Local Git checkout used as the build source. |
| `ARCHIVE_WRAPPER_SHA` | `cd42a203ac6b43d24d9fbd57c323ecd52ea52bd5` | Exact commit archived and built. |
| `ARCHIVE_WRAPPER_IMAGE` | `archive-wrapper:lightnet` | Local image name written by the build. |

### Pulsar and bridge

| Variable | Default | Effect |
| --- | --- | --- |
| `PULSAR_DOCKER_PROJECT` | `pulsar-testnet-N` | Compose project and ownership marker identity. |
| `PULSAR_DOCKER_STATE_ROOT` | `<repository>/.docker` | Parent directory for generated project state. |
| `PULSAR_DOCKER_IMAGE` | `pulsar-chain:local-N-validators` | Image name used by setup and validators. |
| `VALIDATOR_STARTUP_TIMEOUT` | `600` | Compose startup wait timeout in seconds. |
| `BRIDGE_CONFIRMATION_DEPTH` | `3` | Confirmation depth written to validator configs. |
| `BRIDGE_START_BLOCK_HEIGHT` | `1` | Initial Mina archive height queried by the bridge. |
| `BRIDGE_MAX_BLOCK_RANGE` | `1000` | Maximum Mina block range per bridge query. |
| `E2E_USER_MINA_PRIV_KEY` | Unset | Optional local test fixture Mina private key. |
| `E2E_MIN_GAS_PRICE` | Unset | Optional local test fixture minimum gas price. |

`PULSAR_DOCKER_PROJECT` and `PULSAR_DOCKER_STATE_ROOT` form the deployment
identity. Use the same values for subsequent deploy, stop, and reset commands.

## Stop and reset

Export the deployment identity used during startup before lifecycle commands:

```bash
export PULSAR_DOCKER_PROJECT=pulsar-testnet-3
export PULSAR_DOCKER_STATE_ROOT="$PWD/.docker"
```

Stop Pulsar validators and archive-wrapper while preserving their named volumes
and generated metadata:

```bash
./scripts/docker_testnet.sh down 3
```

Remove the selected project's containers, validator homes, archive-wrapper
LevelDB volume, network, and generated metadata:

```bash
./scripts/docker_testnet.sh reset 3
```

Both commands verify the project ownership marker. Neither command removes the
standalone Lightnet container or Docker images.

To pause Lightnet without deleting its database:

```bash
docker stop mina-local-lightnet
```

Restart it before the next deployment to reuse that database. If it remains
stopped, the next deployment treats the owned container as stale and removes it
with its anonymous volumes before creating a fresh Lightnet.

Before manually deleting Lightnet, verify its ownership label:

```bash
docker inspect \
  --format '{{ index .Config.Labels "io.node101.pulsar.local-testnet" }}' \
  mina-local-lightnet
docker rm -fv mina-local-lightnet
```

Run the removal only when the inspect command prints `mina-lightnet`. Change the
container name in these commands when `LIGHTNET_CONTAINER` was overridden.

## Regression tests

Run the script tests without mutating real Docker or Git state:

```bash
make test-scripts
```

The deployment tests use fake Docker, Git, and Curl executables with temporary
home and state directories. They cover cleanup ownership, unrelated-resource
preservation, Lightnet selection, invalid inputs, cleanup failure recovery, the
pinned image, overrides, and Linux host-gateway generation.
