package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	minasignature "github.com/node101-io/mina-signer-go/signature"
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
	fmt.Fprintln(out, "usage: devtools <command> [args]")
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

	priv := keys.PrivateKey{Value: new(big.Int).SetBytes(keyBytes)}
	pub := priv.ToPublicKey()
	bz, err := pub.Marshal()
	if err != nil {
		return fmt.Errorf("marshal mina public key: %w", err)
	}

	fmt.Fprintln(stdout, base64.StdEncoding.EncodeToString(bz))
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		*grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *grpcAddr, err)
	}
	defer conn.Close()

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
	var publicKey keys.PublicKey
	if err := publicKey.Unmarshal(minaPublicKey); err != nil {
		return fmt.Errorf("decode mina public key: %w", err)
	}

	var signature minasignature.Signature
	if err := signature.UnmarshalBytes(signatureBytes); err != nil {
		return fmt.Errorf("decode vote extension signature: %w", err)
	}

	messageHash, err := hashVoteExtBody(body)
	if err != nil {
		return err
	}

	if !publicKey.VerifyFieldElement(&signature, messageHash, networkID) {
		return fmt.Errorf("signature does not verify")
	}

	return nil
}

func hashVoteExtBody(body *votepersistencetypes.VoteExtBody) (*big.Int, error) {
	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)
	if poseidonHash == nil {
		return nil, fmt.Errorf("create poseidon hash")
	}

	innerHash := poseidonHash.Hash([]*big.Int{
		new(big.Int).SetBytes(body.GetNextValidatorSetHash()),
		new(big.Int).SetBytes(body.GetCurrentStateRoot()),
		big.NewInt(body.GetCurrentBlockHeight()),
	})
	if innerHash == nil {
		return nil, fmt.Errorf("hash vote extension body fields")
	}

	messageHash := poseidonHash.Hash([]*big.Int{
		innerHash,
		new(big.Int).SetBytes([]byte(body.GetActionsReducedRoot())),
	})
	if messageHash == nil {
		return nil, fmt.Errorf("hash vote extension reduced root")
	}

	return messageHash, nil
}
