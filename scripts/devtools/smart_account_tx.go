package main

import (
	"context"
	stdlibed25519 "crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cosmossdk.io/core/address"
	txsigning "cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/client"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	appante "github.com/node101-io/pulsar-chain/app/ante"
	antetypes "github.com/node101-io/pulsar-chain/app/ante/types"
	smartaccountstypes "github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const pulsarAccountPrefix = "pulsar"

type smartAccountTxResult struct {
	TxHash    string `json:"tx_hash"`
	Height    int64  `json:"height"`
	Code      uint32 `json:"code"`
	Codespace string `json:"codespace,omitempty"`
	RawLog    string `json:"raw_log,omitempty"`
	GasWanted int64  `json:"gas_wanted"`
	GasUsed   int64  `json:"gas_used"`
}

func runSendSmartAccountTx(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("send-smart-account-tx", flag.ContinueOnError)
	fs.SetOutput(stdout)
	grpcAddr := fs.String("grpc-addr", "127.0.0.1:9090", "Pulsar gRPC endpoint")
	chainID := fs.String("chain-id", "mytestnet", "Pulsar chain id")
	from := fs.String("from", "", "smart account's canonical Pulsar account address")
	to := fs.String("to", "", "recipient Pulsar account address")
	identityValue := fs.String("identity", "", "32-byte smart account identity, encoded as hex or base64")
	sessionKeyFile := fs.String("session-key-file", "", "file containing a 32-byte Ed25519 seed or 64-byte private key, encoded as hex or base64")
	amountValue := fs.String("amount", "1pmina", "coins to send")
	feeValue := fs.String("fees", "100pmina", "transaction fees")
	gas := fs.Uint64("gas", 200000, "transaction gas limit")
	memo := fs.String("memo", "smart-account-local-test", "transaction memo")
	wait := fs.Bool("wait", true, "wait for the transaction to be included in a block")
	timeout := fs.Duration("timeout", 30*time.Second, "query, broadcast, and inclusion timeout")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *from == "" || *to == "" || *identityValue == "" || *sessionKeyFile == "" {
		return errors.New("--from, --to, --identity, and --session-key-file are required")
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

	identity, err := decodeHexOrBase64(*identityValue)
	if err != nil {
		return fmt.Errorf("decode --identity: %w", err)
	}
	if len(identity) != smartaccountstypes.IdentitySize {
		return fmt.Errorf("invalid identity length: got %d bytes, want %d", len(identity), smartaccountstypes.IdentitySize)
	}

	sessionPrivateKey, err := readSessionPrivateKey(*sessionKeyFile)
	if err != nil {
		return err
	}

	amount, err := sdk.ParseCoinsNormalized(*amountValue)
	if err != nil {
		return fmt.Errorf("invalid --amount %q: %w", *amountValue, err)
	}
	if amount.Empty() || !amount.IsAllPositive() {
		return fmt.Errorf("invalid --amount %q: amount must contain positive coins", *amountValue)
	}
	fees, err := sdk.ParseCoinsNormalized(*feeValue)
	if err != nil {
		return fmt.Errorf("invalid --fees %q: %w", *feeValue, err)
	}
	if !fees.IsValid() || fees.IsAnyNegative() {
		return fmt.Errorf("invalid --fees %q: fees must contain non-negative valid coins", *feeValue)
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

	accountResponse, err := authtypes.NewQueryClient(conn).AccountInfo(ctx, &authtypes.QueryAccountInfoRequest{
		Address: *from,
	})
	if err != nil {
		return fmt.Errorf("query account %s: %w", *from, err)
	}
	account := accountResponse.GetInfo()
	if account == nil {
		return fmt.Errorf("account query returned no account info for %s", *from)
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
	txBuilder.SetMemo(*memo)

	extendedBuilder, ok := txBuilder.(client.ExtendedTxBuilder)
	if !ok {
		return errors.New("tx builder does not support extension options")
	}
	extension := &antetypes.TxAuthModeExtension{
		TxAuthMode:           antetypes.TX_AUTH_MODE_SMART_ACCOUNT,
		SmartAccountIdentity: identity,
	}
	extensionBytes, err := gogoproto.Marshal(extension)
	if err != nil {
		return fmt.Errorf("marshal smart account auth extension: %w", err)
	}
	extendedBuilder.SetExtensionOptions(&codectypes.Any{
		TypeUrl: "/" + gogoproto.MessageName(extension),
		Value:   extensionBytes,
	})

	placeholder := signingtypes.SignatureV2{
		PubKey: sessionPrivateKey.PubKey(),
		Data: &signingtypes.SingleSignatureData{
			SignMode: signingtypes.SignMode_SIGN_MODE_DIRECT,
		},
		Sequence: account.Sequence,
	}
	if err := txBuilder.SetSignatures(placeholder); err != nil {
		return fmt.Errorf("set placeholder signature: %w", err)
	}

	signature, err := clienttx.SignWithPrivKey(
		ctx,
		signingtypes.SignMode_SIGN_MODE_DIRECT,
		authsigning.SignerData{
			Address:       *from,
			ChainID:       *chainID,
			AccountNumber: account.AccountNumber,
			Sequence:      account.Sequence,
		},
		txBuilder,
		sessionPrivateKey,
		txConfig,
		account.Sequence,
	)
	if err != nil {
		return fmt.Errorf("sign smart account transaction: %w", err)
	}
	if err := txBuilder.SetSignatures(signature); err != nil {
		return fmt.Errorf("set signed transaction signature: %w", err)
	}

	txBytes, err := txConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return fmt.Errorf("encode signed transaction: %w", err)
	}

	txClient := txtypes.NewServiceClient(conn)
	broadcastResponse, err := txClient.BroadcastTx(ctx, &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	})
	if err != nil {
		return fmt.Errorf("broadcast transaction: %w", err)
	}
	checkTx := broadcastResponse.GetTxResponse()
	if checkTx == nil {
		return errors.New("broadcast returned no transaction response")
	}
	if checkTx.Code != 0 {
		return fmt.Errorf("CheckTx rejected transaction (codespace=%s code=%d): %s", checkTx.Codespace, checkTx.Code, checkTx.RawLog)
	}
	if !*wait {
		return writeSmartAccountTxResult(stdout, checkTx)
	}

	for {
		included, queryErr := txClient.GetTx(ctx, &txtypes.GetTxRequest{Hash: checkTx.TxHash})
		if queryErr == nil && included.GetTxResponse() != nil {
			response := included.GetTxResponse()
			if response.Code != 0 {
				return fmt.Errorf("DeliverTx failed (tx_hash=%s codespace=%s code=%d): %s", response.TxHash, response.Codespace, response.Code, response.RawLog)
			}
			return writeSmartAccountTxResult(stdout, response)
		}
		if queryErr != nil && status.Code(queryErr) != codes.NotFound {
			return fmt.Errorf("query transaction %s: %w", checkTx.TxHash, queryErr)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("transaction %s passed CheckTx but was not found before timeout: %w", checkTx.TxHash, ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func newSmartAccountTxConfig(addressCodec address.Codec) (client.TxConfig, error) {
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: gogoproto.HybridResolver,
		SigningOptions: txsigning.Options{
			AddressCodec:          addressCodec,
			ValidatorAddressCodec: authcodec.NewBech32Codec(pulsarAccountPrefix + "valoper"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create interface registry: %w", err)
	}
	std.RegisterInterfaces(interfaceRegistry)
	auth.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	appante.RegisterInterfaces(interfaceRegistry)

	return authtx.NewTxConfig(codec.NewProtoCodec(interfaceRegistry), authtx.DefaultSignModes), nil
}

func readSessionPrivateKey(path string) (*ed25519.PrivKey, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --session-key-file: %w", err)
	}
	keyBytes, err := decodeHexOrBase64(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode --session-key-file: %w", err)
	}

	switch len(keyBytes) {
	case ed25519.SeedSize:
		return &ed25519.PrivKey{Key: stdlibed25519.NewKeyFromSeed(keyBytes)}, nil
	case ed25519.PrivKeySize:
		seedDerived := stdlibed25519.NewKeyFromSeed(keyBytes[:ed25519.SeedSize])
		if !stdlibed25519.PrivateKey(keyBytes).Equal(seedDerived) {
			return nil, errors.New("64-byte session private key contains a public-key suffix that does not match its seed")
		}
		return &ed25519.PrivKey{Key: append([]byte(nil), keyBytes...)}, nil
	default:
		return nil, fmt.Errorf("invalid session private key length: got %d bytes, want %d-byte seed or %d-byte private key", len(keyBytes), ed25519.SeedSize, ed25519.PrivKeySize)
	}
}

func decodeHexOrBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("value is empty")
	}
	hexValue := strings.TrimPrefix(value, "0x")
	if decoded, err := hex.DecodeString(hexValue); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return nil, errors.New("value is neither valid hex nor standard base64")
}

func writeSmartAccountTxResult(out io.Writer, txResponse *sdk.TxResponse) error {
	return json.NewEncoder(out).Encode(smartAccountTxResult{
		TxHash:    txResponse.TxHash,
		Height:    txResponse.Height,
		Code:      txResponse.Code,
		Codespace: txResponse.Codespace,
		RawLog:    txResponse.RawLog,
		GasWanted: txResponse.GasWanted,
		GasUsed:   txResponse.GasUsed,
	})
}
