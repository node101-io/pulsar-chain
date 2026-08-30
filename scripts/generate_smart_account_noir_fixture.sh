#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
CIRCUIT_DIR="$REPO_ROOT/testdata/smartaccounts/noir/bb-5.2.0"
TOOL_CACHE="${SMART_ACCOUNT_TOOL_CACHE:-$REPO_ROOT/.cache/smartaccounts}"
NARGO_VERSION="1.0.0-beta.25"
BB_VERSION="5.2.0"
BN254_MODULUS="21888242871839275222246405745257275088548364400416034343698204186575808495617"
EXPECTED_VK_HASH="1feb48cba9a74abcc6639cc79d49dc8e81ed9f4e791c371bdf6a5aa04b296eeb"

usage() {
  cat <<'EOF'
usage: scripts/generate_smart_account_noir_fixture.sh \
  --output-dir DIR \
  --session-private-key HEX \
  --session-public-key HEX \
  --expires-at-height UINT64 \
  --identity HEX \
  --account-address HEX

Generates proof, public_inputs, and vk for the local smart-account E2E circuit.
Pinned Nargo and Barretenberg binaries are downloaded into a gitignored cache
when SMART_ACCOUNT_NARGO_BIN and SMART_ACCOUNT_BB_BIN are not provided.
EOF
}

fail() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "missing required command: $1"
  fi
}

OUTPUT_DIR=""
SESSION_PRIVATE_KEY=""
SESSION_PUBLIC_KEY=""
EXPIRES_AT_HEIGHT=""
IDENTITY=""
ACCOUNT_ADDRESS=""

while (( $# > 0 )); do
  case "$1" in
    --output-dir) OUTPUT_DIR="${2:-}"; shift 2 ;;
    --session-private-key) SESSION_PRIVATE_KEY="${2:-}"; shift 2 ;;
    --session-public-key) SESSION_PUBLIC_KEY="${2:-}"; shift 2 ;;
    --expires-at-height) EXPIRES_AT_HEIGHT="${2:-}"; shift 2 ;;
    --identity) IDENTITY="${2:-}"; shift 2 ;;
    --account-address) ACCOUNT_ADDRESS="${2:-}"; shift 2 ;;
    -h | --help) usage; exit 0 ;;
    *) usage >&2; fail "unknown argument: $1" ;;
  esac
done

[[ -n "$OUTPUT_DIR" ]] || fail "--output-dir is required"
[[ -n "$SESSION_PRIVATE_KEY" ]] || fail "--session-private-key is required"
[[ -n "$SESSION_PUBLIC_KEY" ]] || fail "--session-public-key is required"
[[ -n "$EXPIRES_AT_HEIGHT" ]] || fail "--expires-at-height is required"
[[ -n "$IDENTITY" ]] || fail "--identity is required"
[[ -n "$ACCOUNT_ADDRESS" ]] || fail "--account-address is required"

require_cmd curl
require_cmd python3
require_cmd tar

case "$(uname -s):$(uname -m)" in
  Darwin:arm64 | Darwin:aarch64)
    NARGO_ASSET="nargo-aarch64-apple-darwin.tar.gz"
    NARGO_SHA256="63ed453d09a65bfc78eef63252423126959d41c85de0da0cf54289c5e266ceb5"
    BB_ASSET="barretenberg-arm64-darwin.tar.gz"
    BB_SHA256="54be7645c83ac762a38a696cfe0c1495223a4f931b82720bc9e2e47f9a43bf86"
    ;;
  Darwin:x86_64)
    NARGO_ASSET="nargo-x86_64-apple-darwin.tar.gz"
    NARGO_SHA256="660567c645f841389b1e37ddebd4f6c75417763018ea034e08b28338bf27673c"
    BB_ASSET="barretenberg-amd64-darwin.tar.gz"
    BB_SHA256="abf46d12a1a89ae7dc62cf1a84c82873696e1ee6cf925ce241402c0f45736b4b"
    ;;
  Linux:aarch64 | Linux:arm64)
    NARGO_ASSET="nargo-aarch64-unknown-linux-gnu.tar.gz"
    NARGO_SHA256="4e86553af99e87c047bae40f1315234709e8d815f6158f3b8a00bf68f512a2b7"
    BB_ASSET="barretenberg-arm64-linux.tar.gz"
    BB_SHA256="5bdc0552865428ea50d81f4da25b5aa372f45cce33baa0419cbddcd532bdbf30"
    ;;
  Linux:x86_64)
    NARGO_ASSET="nargo-x86_64-unknown-linux-gnu.tar.gz"
    NARGO_SHA256="bf3410ab94933a4aebd1f988b67ae974c6c227f9456ed0f1e4a3716bb8a30fe9"
    BB_ASSET="barretenberg-amd64-linux.tar.gz"
    BB_SHA256="17ab8476961728cdc5c69b6c4ff427c9092cef11d1e0b0166929a0417dfa7cfb"
    ;;
  *) fail "unsupported prover platform: $(uname -s) $(uname -m)" ;;
esac

download_tool() {
  local name="$1"
  local version="$2"
  local asset="$3"
  local expected_sha256="$4"
  local url="$5"
  local destination="$TOOL_CACHE/$name-$version/$name"

  if [[ -x "$destination" ]]; then
    printf '%s\n' "$destination"
    return
  fi

  local download_dir
  download_dir="$(mktemp -d "${TMPDIR:-/tmp}/pulsar-$name.XXXXXX")"
  local archive="$download_dir/$asset"

  echo "==> Downloading $name $version" >&2
  if ! curl --fail --location --retry 3 "$url" --output "$archive"; then
    fail "failed to download $name $version"
  fi
  python3 - "$archive" "$expected_sha256" <<'PY'
import hashlib
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
expected = sys.argv[2]
actual = hashlib.sha256(path.read_bytes()).hexdigest()
if actual != expected:
    raise SystemExit(f"SHA-256 mismatch for {path.name}: got {actual}, want {expected}")
PY

  mkdir -p "$(dirname -- "$destination")"
  if ! tar -xzf "$archive" -C "$(dirname -- "$destination")"; then
    fail "failed to extract $name $version"
  fi
  [[ -x "$destination" ]] || fail "$name archive did not contain an executable named $name"
  printf '%s\n' "$destination"
}

NARGO_BIN="${SMART_ACCOUNT_NARGO_BIN:-}"
if [[ -z "$NARGO_BIN" ]]; then
  NARGO_BIN="$(download_tool \
    nargo "$NARGO_VERSION" "$NARGO_ASSET" "$NARGO_SHA256" \
    "https://github.com/noir-lang/noir/releases/download/v${NARGO_VERSION}/${NARGO_ASSET}")"
fi

BB_BIN="${SMART_ACCOUNT_BB_BIN:-}"
if [[ -z "$BB_BIN" ]]; then
  BB_BIN="$(download_tool \
    bb "$BB_VERSION" "$BB_ASSET" "$BB_SHA256" \
    "https://github.com/AztecProtocol/aztec-packages/releases/download/v${BB_VERSION}/${BB_ASSET}")"
fi

[[ -x "$NARGO_BIN" ]] || fail "Nargo binary is not executable: $NARGO_BIN"
[[ -x "$BB_BIN" ]] || fail "Barretenberg binary is not executable: $BB_BIN"
"$NARGO_BIN" --version | grep -F "$NARGO_VERSION" >/dev/null || \
  fail "Nargo must be version $NARGO_VERSION"
[[ "$("$BB_BIN" --version)" == "$BB_VERSION" ]] || \
  fail "Barretenberg must be version $BB_VERSION"

python3 - \
  "$SESSION_PRIVATE_KEY" \
  "$SESSION_PUBLIC_KEY" \
  "$EXPIRES_AT_HEIGHT" \
  "$IDENTITY" \
  "$ACCOUNT_ADDRESS" \
  "$BN254_MODULUS" <<'PY'
import sys

private_key_hex, public_key_hex, expiry_text, identity_hex, address_hex, modulus_text = sys.argv[1:]

def decode(name, value, expected_length):
    try:
        decoded = bytes.fromhex(value)
    except ValueError as error:
        raise SystemExit(f"{name} is not valid hex: {error}") from error
    if len(decoded) != expected_length:
        raise SystemExit(f"{name} must be {expected_length} bytes, got {len(decoded)}")
    return decoded

decode("session private key", private_key_hex, 32)
public_key = decode("session public key", public_key_hex, 32)
identity = decode("identity", identity_hex, 32)
decode("account address", address_hex, 20)

try:
    expiry = int(expiry_text)
except ValueError as error:
    raise SystemExit("expires-at-height must be an integer") from error
if not 0 < expiry < 2**64:
    raise SystemExit("expires-at-height must be a non-zero uint64")

modulus = int(modulus_text)
for name, value in (("session public key", public_key), ("identity", identity)):
    if int.from_bytes(value, "big") >= modulus:
        raise SystemExit(f"{name} is not a canonical BN254 field element")
PY

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulsar-smart-account-proof.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
mkdir -p "$WORK_DIR/src"
cp "$CIRCUIT_DIR/Nargo.toml" "$WORK_DIR/"
cp "$CIRCUIT_DIR/src/main.nr" "$WORK_DIR/src/main.nr"

python3 - \
  "$WORK_DIR/Prover.toml" \
  "$SESSION_PUBLIC_KEY" \
  "$EXPIRES_AT_HEIGHT" \
  "$IDENTITY" \
  "$ACCOUNT_ADDRESS" <<'PY'
import pathlib
import sys

path, public_key, expiry, identity, address = sys.argv[1:]
pathlib.Path(path).write_text(
    f'session_public_key = "0x{public_key}"\n'
    f'expires_at_height = "{expiry}"\n'
    f'identity = "0x{identity}"\n'
    f'account_address = "0x{address}"\n'
    'witness = "1"\n',
    encoding="utf-8",
)
PY

echo "==> Executing the smart-account Noir circuit"
(
  cd "$WORK_DIR"
  "$NARGO_BIN" execute smart_account_witness
)

mkdir -p "$WORK_DIR/generated" "$TOOL_CACHE/bb-crs" "$OUTPUT_DIR"
echo "==> Generating the UltraHonk proof and verification key"
"$BB_BIN" prove \
  -s ultra_honk \
  -b "$WORK_DIR/target/smart_account_e2e.json" \
  -w "$WORK_DIR/target/smart_account_witness.gz" \
  -o "$WORK_DIR/generated" \
  --write_vk \
  --oracle_hash poseidon2 \
  --crs_path "$TOOL_CACHE/bb-crs"

"$BB_BIN" verify \
  -s ultra_honk \
  -p "$WORK_DIR/generated/proof" \
  -k "$WORK_DIR/generated/vk" \
  -i "$WORK_DIR/generated/public_inputs" \
  --oracle_hash poseidon2 \
  --crs_path "$TOOL_CACHE/bb-crs"

ACTUAL_VK_HASH="$(python3 - "$WORK_DIR/generated/vk" <<'PY'
import hashlib
import pathlib
import sys

print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())
PY
)"
if [[ "$ACTUAL_VK_HASH" != "$EXPECTED_VK_HASH" ]]; then
  fail "generated verification-key hash is $ACTUAL_VK_HASH, expected $EXPECTED_VK_HASH"
fi

python3 - \
  "$WORK_DIR/generated/public_inputs" \
  "$SESSION_PUBLIC_KEY" \
  "$EXPIRES_AT_HEIGHT" \
  "$IDENTITY" \
  "$ACCOUNT_ADDRESS" <<'PY'
import pathlib
import sys

path, public_key, expiry_text, identity, address = sys.argv[1:]
expected = (
    bytes.fromhex(public_key)
    + int(expiry_text).to_bytes(32, "big")
    + bytes.fromhex(identity)
    + bytes.fromhex(address).rjust(32, b"\0")
)
actual = pathlib.Path(path).read_bytes()
if actual != expected:
    raise SystemExit(
        "generated public_inputs do not match the smartaccounts wire encoding"
    )
PY

install -m 0644 "$WORK_DIR/generated/proof" "$OUTPUT_DIR/proof"
install -m 0644 "$WORK_DIR/generated/public_inputs" "$OUTPUT_DIR/public_inputs"
install -m 0644 "$WORK_DIR/generated/vk" "$OUTPUT_DIR/vk"
printf '%s\n' "$SESSION_PRIVATE_KEY" >"$OUTPUT_DIR/session-key.hex"

python3 - \
  "$OUTPUT_DIR/fixture.json" \
  "$IDENTITY" \
  "$SESSION_PUBLIC_KEY" \
  "$EXPIRES_AT_HEIGHT" \
  "$ACCOUNT_ADDRESS" <<'PY'
import json
import pathlib
import sys

path, identity, public_key, expiry, account_address = sys.argv[1:]
pathlib.Path(path).write_text(
    json.dumps(
        {
            "identity_hex": identity,
            "session_public_key_hex": public_key,
            "expires_at_height": int(expiry),
            "account_address_hex": account_address,
        },
        indent=2,
    )
    + "\n",
    encoding="utf-8",
)
PY

echo "==> Smart-account Noir fixture generated in $OUTPUT_DIR"
