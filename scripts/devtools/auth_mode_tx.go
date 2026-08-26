package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/privatekey"
	antetypes "github.com/node101-io/pulsar-chain/app/ante/types"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	minaTxSigningChallengePrefix  = "pulsar-tx-auth-v1"
	minaTxSigningChallengeVersion = byte(1)
)

type minaRegistrationMaterial struct {
	MinaPublicKey string `json:"mina_public_key"`
	MinaSignature string `json:"mina_signature"`
}

func runMinaRegistrationMaterial(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("mina-registration-material", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	chainID := fs.String("chain-id", "", "chain ID")
	cosmosPublicKeyValue := fs.String("cosmos-public-key", "", "Cosmos public key in hex or base64")
	minaPrivateKeyValue := fs.String("mina-private-key", "", "Mina private key in hex or base64")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *chainID == "" || *cosmosPublicKeyValue == "" || *minaPrivateKeyValue == "" {
		return errors.New("--chain-id, --cosmos-public-key, and --mina-private-key are required")
	}

	cosmosPublicKey, err := decodeHexOrBase64(*cosmosPublicKeyValue)
	if err != nil {
		return fmt.Errorf("decode --cosmos-public-key: %w", err)
	}
	minaPrivateKey, err := parseMinaPrivateKey(*minaPrivateKeyValue)
	if err != nil {
		return err
	}
	minaPublicKey, err := minaPrivateKey.ToPublicKey()
	if err != nil {
		return fmt.Errorf("derive Mina public key: %w", err)
	}
	minaPublicKeyBytes := minaPublicKey.Bytes()

	challenge, err := keyregistrytypes.BuildKeySigningChallenge(keyregistrytypes.KeySigningChallengeInput{
		ChainID:          *chainID,
		Operation:        keyregistrytypes.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
		ActorType:        keyregistrytypes.ActorType_ACTOR_TYPE_USER,
		CosmosPublicKey:  cosmosPublicKey,
		NewMinaPublicKey: minaPublicKeyBytes,
	})
	if err != nil {
		return fmt.Errorf("build Mina registration challenge: %w", err)
	}
	minaSignature, err := minaPrivateKey.SignFieldElement(challenge)
	if err != nil {
		return fmt.Errorf("sign Mina registration challenge: %w", err)
	}

	return json.NewEncoder(stdout).Encode(minaRegistrationMaterial{
		MinaPublicKey: hex.EncodeToString(minaPublicKeyBytes),
		MinaSignature: hex.EncodeToString(minaSignature.Bytes()),
	})
}

func runSendMinaAuthModeTx(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("send-mina-auth-mode-tx", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	grpcAddr := fs.String("grpc-addr", "127.0.0.1:9090", "chain gRPC endpoint")
	chainID := fs.String("chain-id", "mytestnet", "chain ID")
	from := fs.String("from", "", "sender account address")
	to := fs.String("to", "", "recipient account address")
	minaPrivateKeyValue := fs.String("mina-private-key", "", "Mina private key in hex or base64")
	amountValue := fs.String("amount", "1pmina", "amount to send")
	feeValue := fs.String("fees", "100pmina", "transaction fees")
	gas := fs.Uint64("gas", 200000, "gas limit")
	timeout := fs.Duration("timeout", 30*time.Second, "query and inclusion timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *from == "" || *to == "" || *minaPrivateKeyValue == "" {
		return errors.New("--from, --to, and --mina-private-key are required")
	}
	if *chainID == "" || *grpcAddr == "" || *gas == 0 || *timeout <= 0 {
		return errors.New("--chain-id, --grpc-addr, --gas, and --timeout must be non-zero")
	}

	addressCodec := authcodec.NewBech32Codec(pulsarAccountPrefix)
	if _, err := addressCodec.StringToBytes(*from); err != nil {
		return fmt.Errorf("invalid --from address: %w", err)
	}
	if _, err := addressCodec.StringToBytes(*to); err != nil {
		return fmt.Errorf("invalid --to address: %w", err)
	}
	minaPrivateKey, err := parseMinaPrivateKey(*minaPrivateKeyValue)
	if err != nil {
		return err
	}
	amount, err := sdk.ParseCoinsNormalized(*amountValue)
	if err != nil || amount.Empty() || !amount.IsAllPositive() {
		return fmt.Errorf("invalid --amount %q", *amountValue)
	}
	fees, err := sdk.ParseCoinsNormalized(*feeValue)
	if err != nil || !fees.IsValid() || fees.IsAnyNegative() {
		return fmt.Errorf("invalid --fees %q", *feeValue)
	}

	txConfig, err := newSmartAccountTxConfig(addressCodec)
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(*grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create gRPC client for %s: %w", *grpcAddr, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	accountResponse, err := authtypes.NewQueryClient(conn).AccountInfo(ctx, &authtypes.QueryAccountInfoRequest{Address: *from})
	if err != nil {
		return fmt.Errorf("query account %s: %w", *from, err)
	}
	account := accountResponse.GetInfo()
	if account == nil {
		return fmt.Errorf("account query returned no account info for %s", *from)
	}
	if account.PubKey == nil || account.PubKey.TypeUrl != "/cosmos.crypto.secp256k1.PubKey" {
		return fmt.Errorf("account %s has no Cosmos public key", *from)
	}
	cosmosPublicKey := &secp256k1.PubKey{}
	if err := gogoproto.Unmarshal(account.PubKey.Value, cosmosPublicKey); err != nil {
		return fmt.Errorf("decode Cosmos public key for %s: %w", *from, err)
	}

	txBuilder := txConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(&banktypes.MsgSend{
		FromAddress: *from,
		ToAddress:   *to,
		Amount:      amount,
	}); err != nil {
		return fmt.Errorf("set MsgSend: %w", err)
	}
	txBuilder.SetFeeAmount(fees)
	txBuilder.SetGasLimit(*gas)

	extendedBuilder, ok := txBuilder.(client.ExtendedTxBuilder)
	if !ok {
		return errors.New("tx builder does not support extension options")
	}
	extensionBytes, err := gogoproto.Marshal(&antetypes.TxAuthModeExtension{
		TxAuthMode: antetypes.TX_AUTH_MODE_MINA,
	})
	if err != nil {
		return fmt.Errorf("marshal Mina auth extension: %w", err)
	}
	extendedBuilder.SetExtensionOptions(&codectypes.Any{
		TypeUrl: "/" + gogoproto.MessageName(&antetypes.TxAuthModeExtension{}),
		Value:   extensionBytes,
	})

	signMode := signingtypes.SignMode_SIGN_MODE_DIRECT
	placeholder := signingtypes.SignatureV2{
		PubKey:   cosmosPublicKey,
		Data:     &signingtypes.SingleSignatureData{SignMode: signMode},
		Sequence: account.Sequence,
	}
	if err := txBuilder.SetSignatures(placeholder); err != nil {
		return fmt.Errorf("set placeholder signature: %w", err)
	}
	signBytes, err := authsigning.GetSignBytesAdapter(
		ctx,
		txConfig.SignModeHandler(),
		signMode,
		authsigning.SignerData{
			Address:       *from,
			ChainID:       *chainID,
			AccountNumber: account.AccountNumber,
			Sequence:      account.Sequence,
		},
		txBuilder.GetTx(),
	)
	if err != nil {
		return fmt.Errorf("build Mina sign bytes: %w", err)
	}
	challenge, err := buildMinaTxSigningChallenge(signBytes)
	if err != nil {
		return err
	}
	minaSignature, err := minaPrivateKey.SignFieldElement(challenge)
	if err != nil {
		return fmt.Errorf("sign Mina transaction challenge: %w", err)
	}
	if err := txBuilder.SetSignatures(signingtypes.SignatureV2{
		PubKey: cosmosPublicKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signMode,
			Signature: minaSignature.Bytes(),
		},
		Sequence: account.Sequence,
	}); err != nil {
		return fmt.Errorf("set Mina signature: %w", err)
	}

	txBytes, err := txConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return fmt.Errorf("encode Mina transaction: %w", err)
	}
	broadcastResponse, err := txtypes.NewServiceClient(conn).BroadcastTx(ctx, &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	})
	if err != nil {
		return fmt.Errorf("broadcast Mina transaction: %w", err)
	}
	checkTx := broadcastResponse.GetTxResponse()
	if checkTx == nil {
		return errors.New("broadcast returned no transaction response")
	}
	if checkTx.Code != 0 {
		return fmt.Errorf("CheckTx rejected Mina transaction (codespace=%s code=%d): %s", checkTx.Codespace, checkTx.Code, checkTx.RawLog)
	}

	for {
		included, queryErr := txtypes.NewServiceClient(conn).GetTx(ctx, &txtypes.GetTxRequest{Hash: checkTx.TxHash})
		if queryErr == nil && included.GetTxResponse() != nil {
			response := included.GetTxResponse()
			if response.Code != 0 {
				return fmt.Errorf("DeliverTx failed (tx_hash=%s codespace=%s code=%d): %s", response.TxHash, response.Codespace, response.Code, response.RawLog)
			}
			return writeSmartAccountTxResult(stdout, response)
		}
		if queryErr != nil && status.Code(queryErr) != codes.NotFound {
			return fmt.Errorf("query Mina transaction %s: %w", checkTx.TxHash, queryErr)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Mina transaction %s was not included before timeout: %w", checkTx.TxHash, ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func parseMinaPrivateKey(value string) (*privatekey.PrivateKey, error) {
	keyBytes, err := decodeHexOrBase64(value)
	if err != nil {
		return nil, fmt.Errorf("decode Mina private key: %w", err)
	}
	if len(keyBytes) != privatekey.Size() {
		return nil, fmt.Errorf("invalid Mina private key length: got %d bytes, want %d", len(keyBytes), privatekey.Size())
	}
	var rawPrivateKey [32]byte
	copy(rawPrivateKey[:], keyBytes)
	key, err := privatekey.NewPrivateKeyFromBytes(rawPrivateKey, mina.TestNet)
	if err != nil {
		return nil, fmt.Errorf("parse Mina private key: %w", err)
	}
	return key, nil
}

func buildMinaTxSigningChallenge(signBytes []byte) (*minafield.FieldElement, error) {
	if len(signBytes) == 0 {
		return nil, errors.New("empty Mina transaction sign bytes")
	}
	if len(signBytes) > math.MaxUint32 {
		return nil, errors.New("Mina transaction sign bytes exceed uint32 framing")
	}

	payload := bytes.NewBuffer(make([]byte, 0, 5+len(signBytes)))
	payload.WriteByte(minaTxSigningChallengeVersion)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(signBytes)))
	payload.Write(length[:])
	payload.Write(signBytes)
	hash, err := poseidon.NewPoseidon().HashWithPrefix(minaTxSigningChallengePrefix, payload.Bytes())
	if err != nil {
		return nil, fmt.Errorf("hash Mina transaction challenge: %w", err)
	}
	challenge, err := minafield.NewFieldElement(hash)
	if err != nil {
		return nil, fmt.Errorf("decode Mina transaction challenge: %w", err)
	}
	return challenge, nil
}
