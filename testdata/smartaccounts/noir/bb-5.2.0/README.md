# Smart-account Noir E2E circuit

This test-only circuit binds a proof to the four public fields consumed by
`x/smartaccounts` in this exact order:

1. 32-byte Ed25519 session public key
2. expiration block height, encoded as a 32-byte big-endian integer
3. 32-byte identity
4. 20-byte Cosmos account address, left-padded to 32 bytes

The circuit is intentionally limited to exercising the complete local
prover, verifier, verification-module, smartaccounts, and ante-handler flow.
It does not prove ownership of a production identity provider account.

The pinned artifact contract is:

- Nargo `1.0.0-beta.25`
- Barretenberg `5.2.0`
- UltraHonk
- Poseidon2 transcript
- zero knowledge enabled
- IPA accumulation disabled

The canonical `SHA-256(vk)` is:

```text
1feb48cba9a74abcc6639cc79d49dc8e81ed9f4e791c371bdf6a5aa04b296eeb
```

The same value is configured for the local network in `config.yml`.

Generate a proof with `scripts/generate_smart_account_noir_fixture.sh`.
