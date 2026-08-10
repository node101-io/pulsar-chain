# Archive-wrapper deployment

Every Pulsar validator requires a ready archive-wrapper before `pulsard start`
can construct the application or open CometBFT listeners. The wrapper supplies
the finalized Mina action range used by the bridge module, so there is no
wrapper-disabled validator mode.

The Docker testnet generator supports three explicit deployment topologies:

- `shared`: one wrapper and one LevelDB volume serve every validator.
- `per-validator`: each validator has its own wrapper and LevelDB volume.
- `external`: Pulsar connects to a wrapper managed outside the generated Compose
  project.

`ARCHIVE_WRAPPER_MODE` has no default. Operators must select one of these modes
when generating or starting a deployment.

## Runtime contract

Each validator's `app.toml` contains both values:

```toml
[bridge]
wrapper_grpc_address = "archive-wrapper:9095"
wrapper_grpc_transport_mode = "trusted-network"
```

`pulsard start` validates the endpoint and calls the standard gRPC health service
for `query.Query` with a five-second timeout. Only `SERVING` passes startup. The
same check is available to scripts and operators:

```bash
pulsard healthcheck archive-wrapper --home /var/lib/pulsar
```

This check exits silently with status 0 on success and exits non-zero for invalid
configuration, connection errors, timeouts, and non-serving responses. Offline
commands such as genesis generation, export, and query commands do not contact
the wrapper.

The validator container healthcheck combines the wrapper check with the existing
CometBFT RPC, catching-up, positive-height, and block-age checks. Losing the
wrapper after startup makes the validator unhealthy but does not terminate the
Pulsar process. Bridge relayers must wait for wrapper recovery before submitting
`PushNewActions`; other chain activity may continue.

## Shared topology

Shared mode requires an immutable wrapper image reference and a PostgreSQL URI:

```bash
export ARCHIVE_WRAPPER_MODE=shared
export ARCHIVE_WRAPPER_IMAGE=archive-wrapper@sha256:<digest>
export POSTGRES_URI='postgres://user:password@postgres:5432/archive?sslmode=disable'

./scripts/docker_testnet.sh config 3
./scripts/docker_testnet.sh up 3
```

The generated project creates one `archive-wrapper` service and one
`archive-wrapper_data` volume. Validator 1's home is mounted read-only because
the wrapper reads the shared genesis configuration from it. Every validator waits
for the same wrapper healthcheck and uses `archive-wrapper:9095` with
`trusted-network` transport.

This mode has a single wrapper failure domain. If it is unavailable, every
validator becomes unhealthy until that wrapper recovers.

## Per-validator topology

Per-validator mode uses the same required inputs:

```bash
export ARCHIVE_WRAPPER_MODE=per-validator
export ARCHIVE_WRAPPER_IMAGE=archive-wrapper@sha256:<digest>
export POSTGRES_URI='postgres://user:password@postgres:5432/archive?sslmode=disable'

./scripts/docker_testnet.sh config 3
./scripts/docker_testnet.sh up 3
```

The generator creates one wrapper and one LevelDB volume for each validator.
`validatorN` depends only on `archive-wrapper-validatorN` and uses the matching
service endpoint. A LevelDB volume must never be mounted by two running wrapper
instances. PostgreSQL can be shared because each wrapper independently derives
and persists its own cursor.

This topology isolates wrapper process failures. A failed wrapper marks only its
paired validator unhealthy, but bridge transactions must still wait until all
validators used by the deployment have reconciled to the same archive state.

## External topology

External mode creates no wrapper service, config, or LevelDB volume:

```bash
export ARCHIVE_WRAPPER_MODE=external
export ARCHIVE_WRAPPER_EXTERNAL_ADDRESS=archive-wrapper.internal:9095
export ARCHIVE_WRAPPER_EXTERNAL_TRANSPORT_MODE=trusted-network
export ARCHIVE_WRAPPER_EXTERNAL_NETWORK=wrapper-network # optional Compose network

./scripts/docker_testnet.sh config 3
./scripts/docker_testnet.sh up 3
```

When `ARCHIVE_WRAPPER_EXTERNAL_NETWORK` is set, it must already exist and the
validator services join it. Compose cannot express health dependencies on an
external process, so each validator's mandatory start preflight enforces
readiness. Starting the same validator command again after the wrapper becomes
ready succeeds without regenerating its home.

`trusted-network` is an explicit plaintext transport policy for controlled
private networks. It accepts loopback/private literal IPs and valid DNS service
names, then uses insecure gRPC credentials. It does not authenticate the remote
peer and must not be treated as production-safe over public or untrusted
networks. Remote deployment across such a boundary requires a future TLS/mTLS
transport mode.

## Configuration and secrets

Node-specific wrapper values take precedence over global values during testnet
setup:

```text
NODE<N>_WRAPPER_GRPC_ADDRESS
NODE<N>_WRAPPER_GRPC_TRANSPORT_MODE
WRAPPER_GRPC_ADDRESS
WRAPPER_GRPC_TRANSPORT_MODE
config.yml validator app values
```

An explicitly empty override is an error and does not fall back. The generator
writes effective address and transport values to each `app.toml` exactly once.

`POSTGRES_URI` is passed to wrapper containers through the environment and is not
written to generated wrapper config files. Runtime data lives only in named
volumes. Wrapper containers use a read-only root filesystem, a non-root user, and
a private tmpfs for `/run/archive-wrapper`.

Use `down` for a restart that preserves homes and cursors:

```bash
./scripts/docker_testnet.sh down 3
```

Use `reset` only when validator homes, wrapper LevelDB volumes, and generated
runtime files should be removed:

```bash
./scripts/docker_testnet.sh reset 3
```

`down` and `reset` reuse the previously generated Compose file and therefore do
not require the deployment mode or PostgreSQL secret to be supplied again.

## Tested wrapper contract

The integration suite pins archive-wrapper commit
`163a2530a9f1354e23c152f8d14c19524369ab38`. CI checks out that exact revision,
builds its Phase 3 container image, and runs shared, per-validator, and external
end-to-end scenarios. Deployment artifacts should pin a released image digest
produced from the reviewed wrapper revision rather than a mutable tag.

## Container security policy

Container CI scans the actual Pulsar builder stage and final runtime image
separately. Fixable `HIGH` and `CRITICAL` operating-system package
vulnerabilities fail the build for either image. Findings without an available
upstream package fix remain visible without blocking changes that cannot yet
consume a fix.

Go dependency findings are analyzed from source with a pinned `govulncheck`
version inside the builder environment. The scan uses the production `purego`
build tag and targets only the `pulsard` and `pulsar-devtools` executables that
ship in the image. Reachable findings are retained as a report-only SARIF
artifact; tool installation, configuration, or analysis failures still fail CI.
Import-only and module-only informational entries are excluded from the
artifact, and reachable findings are never suppressed automatically.

Report-only status does not establish that a reachable vulnerability is safe.
Dependency upgrades require separate compatibility review against Cosmos SDK,
CometBFT, and the bridge integration before they become blocking policy or are
accepted with documented compensating controls.
