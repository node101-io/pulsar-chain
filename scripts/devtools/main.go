package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/mina-signer-go/publickey"
	minasignature "github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/abci"
	abcitypes "github.com/node101-io/pulsar-chain/abci"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	votepersistencetypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "derive-mina-pub":
		if err := runDeriveMinaPub(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "verify-vote-extensions":
		if err := runVerifyVoteExtensions(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "-h", "--help", "help":
		printUsage(stdout)
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}

	return 0
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "usage: go run -tags=purego ./scripts/devtools <command> [args]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "commands:")
	fmt.Fprintln(out, "  derive-mina-pub <mina-private-key-base64>")
	fmt.Fprintln(out, "  verify-vote-extensions [--grpc-addr addr] [--network-id id] [--timeout duration]")
}

func runDeriveMinaPub(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("derive-mina-pub", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: devtools derive-mina-pub <mina-private-key-base64>")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("decode mina private key: %w", err)
	}

	if len(keyBytes) != privatekey.Size() {
		return fmt.Errorf("invalid mina private key length: got %d bytes, want %d", len(keyBytes), privatekey.Size())
	}

	var rawPrivateKey [32]byte
	copy(rawPrivateKey[:], keyBytes)

	priv, err := privatekey.NewPrivateKeyFromBytes(rawPrivateKey, mina.TestNet)
	if err != nil {
		return fmt.Errorf("parse mina private key: %w", err)
	}
	pub, err := priv.ToPublicKey()
	if err != nil {
		return fmt.Errorf("derive mina public key: %w", err)
	}

	fmt.Fprintln(stdout, base64.StdEncoding.EncodeToString(pub.Bytes()))
	return nil
}

func runVerifyVoteExtensions(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("verify-vote-extensions", flag.ContinueOnError)
	fs.SetOutput(stdout)
	grpcAddr := fs.String("grpc-addr", "localhost:9091", "gRPC endpoint address")
	networkID := fs.String("network-id", "testnet", "Mina signature network id")
	timeout := fs.Duration("timeout", 10*time.Second, "query timeout")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	conn, err := grpc.NewClient(
		*grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("create gRPC client for %s: %w", *grpcAddr, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	votePersistenceClient := votepersistencetypes.NewQueryClient(conn)
	abciClient := abcitypes.NewQueryClient(conn)
	keyClient := keyregistrytypes.NewQueryClient(conn)

	votesResp, err := votePersistenceClient.VoteExtensions(ctx, &votepersistencetypes.QueryVoteExtensionsRequest{})
	if err != nil {
		return fmt.Errorf("query persisted vote extensions: %w", err)
	}
	if len(votesResp.GetVoteExtensions()) == 0 {
		return fmt.Errorf("no persisted vote extensions found at query height %d", votesResp.GetQueryBlockHeight())
	}

	signedStateHeight := votesResp.GetPersistedVoteExtensionsBlockHeight()
	voteExtensionHeight := signedStateHeight + 2
	fmt.Fprintf(stdout, "query_block_height: %d\n", votesResp.GetQueryBlockHeight())
	fmt.Fprintf(stdout, "signed_state_height: %d\n", signedStateHeight)
	fmt.Fprintf(stdout, "abci_vote_extension_height_for_body_query: %d\n", voteExtensionHeight)
	fmt.Fprintf(stdout, "vote_extensions: %d\n\n", len(votesResp.GetVoteExtensions()))

	bodyResp, err := abciClient.VoteExtBodyByHeight(ctx, &abcitypes.QueryVoteExtBodyByHeightRequest{
		VoteExtensionHeight: voteExtensionHeight,
	})
	if err != nil {
		return fmt.Errorf("query vote extension body at height %d: %w", voteExtensionHeight, err)
	}
	body := bodyResp.GetVoteExtBody()
	if body == nil {
		return fmt.Errorf("vote extension body response is empty")
	}
	if body.GetCurrentBlockHeight() != signedStateHeight {
		return fmt.Errorf("body current block height mismatch: got %d, want %d", body.GetCurrentBlockHeight(), signedStateHeight)
	}

	fmt.Fprintf(stdout, "body.current_block_height: %d\n", body.GetCurrentBlockHeight())
	fmt.Fprintf(stdout, "body.current_state_root: %s\n", base64.StdEncoding.EncodeToString(body.GetCurrentStateRoot()))
	fmt.Fprintf(stdout, "body.next_validator_set_hash: %s\n", base64.StdEncoding.EncodeToString(body.GetNextValidatorSetHash()))
	fmt.Fprintf(stdout, "body.actions_reduced_root: %q\n\n", body.GetActionsReducedRoot())

	for i, vote := range votesResp.GetVoteExtensions() {
		if err := verifyStoredVote(ctx, keyClient, body, vote, *networkID); err != nil {
			return fmt.Errorf("vote %d verification failed: %w", i+1, err)
		}

		fmt.Fprintf(
			stdout,
			"vote %d ok: mina_public_key=%s vote_extension=%s\n",
			i+1,
			base64.StdEncoding.EncodeToString(vote.GetMinaPublicKey()),
			base64.StdEncoding.EncodeToString(vote.GetVoteExtension()),
		)
	}

	fmt.Fprintf(stdout, "\nverified %d/%d vote extensions\n", len(votesResp.GetVoteExtensions()), len(votesResp.GetVoteExtensions()))
	return nil
}

func verifyStoredVote(
	ctx context.Context,
	keyClient keyregistrytypes.QueryClient,
	body *votepersistencetypes.VoteExtBody,
	vote *votepersistencetypes.StoredVoteExtension,
	networkID string,
) error {
	if len(vote.GetMinaPublicKey()) == 0 {
		return fmt.Errorf("empty mina public key")
	}
	if len(vote.GetVoteExtension()) == 0 {
		return fmt.Errorf("empty vote extension")
	}

	cosmosKeyResp, err := keyClient.GetValidatorCosmosPubKey(ctx, &keyregistrytypes.QueryGetValidatorCosmosPubKeyRequest{
		ValidatorMinaPubKey: vote.GetMinaPublicKey(),
	})
	if err != nil {
		return fmt.Errorf("query validator cosmos key from mina key: %w", err)
	}
	cosmosKey := cosmosKeyResp.GetValidatorCosmosPubKey()
	if len(cosmosKey) == 0 {
		return fmt.Errorf("keyregistry returned empty validator cosmos key")
	}

	minaKeyResp, err := keyClient.GetValidatorMinaPubKey(ctx, &keyregistrytypes.QueryGetValidatorMinaPubKeyRequest{
		ValidatorCosmosPubKey: cosmosKey,
	})
	if err != nil {
		return fmt.Errorf("query validator mina key from cosmos key: %w", err)
	}
	if !bytes.Equal(minaKeyResp.GetValidatorMinaPubKey(), vote.GetMinaPublicKey()) {
		return fmt.Errorf("keyregistry round trip mismatch")
	}

	if err := verifyVoteExtensionSignature(body, vote.GetMinaPublicKey(), vote.GetVoteExtension(), networkID); err != nil {
		return err
	}

	return nil
}

func verifyVoteExtensionSignature(body *votepersistencetypes.VoteExtBody, minaPublicKey, signatureBytes []byte, networkID string) error {
	publicKey, err := publickey.NewPublicKeyFromBytes(minaPublicKey, mina.NetworkID(networkID))
	if err != nil {
		return fmt.Errorf("decode mina public key: %w", err)
	}

	signature, err := minasignature.NewSignatureFromBytes(signatureBytes)
	if err != nil {
		return fmt.Errorf("decode vote extension signature: %w", err)
	}

	poseidonHash := poseidon.NewPoseidon()
	minaField := field.NewField()

	messageHash, err := hashVoteExtBody(minaField, poseidonHash, *body)
	if err != nil {
		return err
	}

	valid, err := publicKey.VerifyField(signature, messageHash)
	if err != nil {
		return fmt.Errorf("verify signature: %w", err)
	}
	if !valid {
		return fmt.Errorf("signature does not verify")
	}

	return nil
}

func hashVoteExtBody(minaField *field.Field, poseidonHash *poseidon.Poseidon, voteExtBody votepersistencetypes.VoteExtBody) (*field.FieldElement, error) {
	if poseidonHash == nil {
		return nil, abci.ErrVoteExtBodyHashFailed
	}

	if minaField == nil {
		return nil, abci.ErrVoteExtBodyHashFailed
	}

	if voteExtBody.CurrentBlockHeight < 0 {
		return nil, fmt.Errorf("%w: current block height must be non-negative", abci.ErrVoteExtBodyHashFailed)
	}

	validatorSetRoot, err := minaField.FromBytes(voteExtBody.NextValidatorSetHash)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid validator set root: %v", abci.ErrVoteExtBodyHashFailed, err)
	}

	voteExtBodyHash, err := encodeVoteExtBodyForHash(poseidonHash, minaField, voteExtBody.CurrentStateRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", abci.ErrVoteExtBodyHashFailed, err)
	}

	actionsRoot, err := minaField.FromBytesBEReduce([]byte(voteExtBody.ActionsReducedRoot))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid actions reduced root: %v", abci.ErrVoteExtBodyHashFailed, err)
	}

	inner, err := poseidonHash.HashFieldElements(
		validatorSetRoot,
		voteExtBodyHash,
		minaField.FromUint64(uint64(voteExtBody.CurrentBlockHeight)),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", abci.ErrVoteExtBodyHashFailed, err)
	}

	root, err := field.NewFieldElement(actionsRoot.Bytes())
	if err != nil {
		return nil, err
	}

	hash, err := poseidonHash.HashFieldElements(inner, root)
	if err != nil {
		return nil, err
	}

	return hash, nil
}
func encodeVoteExtBodyForHash(
	poseidonHash *poseidon.Poseidon,
	field *field.Field,
	appHash []byte,
) (*field.FieldElement, error) {
	if len(appHash) != 32 {
		return nil, fmt.Errorf("current state root must be 32 bytes")
	}

	hi, err := field.FromBytesBEReduce(appHash[:16])
	if err != nil {
		return nil, err
	}

	lo, err := field.FromBytesBEReduce(appHash[16:])
	if err != nil {
		return nil, err
	}

	hashBytes, err := poseidonHash.HashFieldElements(hi, lo)
	if err != nil {
		return nil, err
	}

	return hashBytes, nil
}
