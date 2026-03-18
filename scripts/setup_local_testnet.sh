set -e

BINARY="pulsar-chaind"
CHAIN_ID="mytestnet"
NODE1_HOME="$HOME/.pulsar-node1"
NODE2_HOME="$HOME/.pulsar-node2"
DENOM="pmina"
NODE1_MINA_PRIV_KEY="ES17xFroE2/QOa9yCLXsQ9sJMeIUVwr2ZXcdWGjNLlM="
NODE2_MINA_PRIV_KEY="PKeRXivUb4gZ/nMKxUK5beEnVJwIrzN71mAf7JVKsng="

echo "==> Cleaning up old data..."
rm -rf $NODE1_HOME $NODE2_HOME

echo "==> Initializing nodes..."
$BINARY init node1 --chain-id $CHAIN_ID --home $NODE1_HOME > /dev/null 2>&1
$BINARY init node2 --chain-id $CHAIN_ID --home $NODE2_HOME > /dev/null 2>&1

echo "==> Setting vote extensions enable height..."
python3 -c "
import json
with open('$NODE1_HOME/config/genesis.json') as f:
    g = json.load(f)
g['consensus']['params']['abci']['vote_extensions_enable_height'] = '2'
with open('$NODE1_HOME/config/genesis.json', 'w') as f:
    json.dump(g, f, indent=2)
"

echo "==> Creating keys (both in node1 keyring)..."
$BINARY keys add validator1 --home $NODE1_HOME --keyring-backend test
$BINARY keys add validator2 --home $NODE1_HOME --keyring-backend test

echo "==> Adding genesis accounts..."
VAL1_ADDR=$($BINARY keys show validator1 --home $NODE1_HOME --keyring-backend test --address)
VAL2_ADDR=$($BINARY keys show validator2 --home $NODE1_HOME --keyring-backend test --address)

echo "VAL1: $VAL1_ADDR"
echo "VAL2: $VAL2_ADDR"

$BINARY genesis add-genesis-account $VAL1_ADDR 1000000000$DENOM --home $NODE1_HOME
$BINARY genesis add-genesis-account $VAL2_ADDR 1000000000$DENOM --home $NODE1_HOME

echo "==> Verifying genesis accounts..."
cat $NODE1_HOME/config/genesis.json | python3 -m json.tool | grep "address"

echo "==> Creating gentx for node1..."
$BINARY genesis gentx validator1 100000000$DENOM \
  --chain-id $CHAIN_ID \
  --home $NODE1_HOME \
  --keyring-backend test \
  --keyring-dir $NODE1_HOME

echo "==> Copying genesis to node2 before node2 gentx..."
cp $NODE1_HOME/config/genesis.json $NODE2_HOME/config/genesis.json

echo "==> Creating gentx for node2..."
$BINARY genesis gentx validator2 100000000$DENOM \
  --chain-id $CHAIN_ID \
  --home $NODE2_HOME \
  --keyring-backend test \
  --keyring-dir $NODE1_HOME

echo "==> Copying node2 gentx to node1..."
cp $NODE2_HOME/config/gentx/*.json $NODE1_HOME/config/gentx/

echo "==> Collecting gentxs..."
$BINARY genesis collect-gentxs --home $NODE1_HOME
$BINARY genesis validate-genesis --home $NODE1_HOME

echo "==> Verifying 2 validators in genesis..."
cat $NODE1_HOME/config/genesis.json | python3 -m json.tool | grep "validator_address"

echo "==> Copying final genesis to node2..."
cp $NODE1_HOME/config/genesis.json $NODE2_HOME/config/genesis.json

echo "==> Getting node IDs..."
NODE1_ID=$($BINARY tendermint show-node-id --home $NODE1_HOME)
NODE2_ID=$($BINARY tendermint show-node-id --home $NODE2_HOME)
echo "Node1 ID: $NODE1_ID"
echo "Node2 ID: $NODE2_ID"

echo "==> Configuring node1..."
sed -i.bak 's|laddr = "tcp://127.0.0.1:26657"|laddr = "tcp://0.0.0.0:26657"|' $NODE1_HOME/config/config.toml
sed -i.bak "s|persistent_peers = \"\"|persistent_peers = \"$NODE2_ID@127.0.0.1:26666\"|" $NODE1_HOME/config/config.toml
sed -i.bak 's|addr_book_strict = true|addr_book_strict = false|' $NODE1_HOME/config/config.toml
sed -i.bak 's|allow_duplicate_ip = false|allow_duplicate_ip = true|' $NODE1_HOME/config/config.toml

echo "==> Configuring node2..."
sed -i.bak 's|laddr = "tcp://127.0.0.1:26657"|laddr = "tcp://0.0.0.0:26667"|' $NODE2_HOME/config/config.toml
sed -i.bak 's|laddr = "tcp://0.0.0.0:26656"|laddr = "tcp://0.0.0.0:26666"|' $NODE2_HOME/config/config.toml
sed -i.bak "s|persistent_peers = \"\"|persistent_peers = \"$NODE1_ID@127.0.0.1:26656\"|" $NODE2_HOME/config/config.toml
sed -i.bak 's|addr_book_strict = true|addr_book_strict = false|' $NODE2_HOME/config/config.toml
sed -i.bak 's|allow_duplicate_ip = false|allow_duplicate_ip = true|' $NODE2_HOME/config/config.toml
sed -i.bak 's|address = "tcp://localhost:1317"|address = "tcp://localhost:1318"|' $NODE2_HOME/config/app.toml
sed -i.bak 's|address = "localhost:9090"|address = "localhost:9091"|' $NODE2_HOME/config/app.toml

sed -i.bak 's|minimum-gas-prices = ""|minimum-gas-prices = "0pmina"|' $NODE1_HOME/config/app.toml
sed -i.bak 's|minimum-gas-prices = ""|minimum-gas-prices = "0pmina"|' $NODE2_HOME/config/app.toml

cat >> ~/.pulsar-node1/config/app.toml << EOF

[vote_extension]
priv_key = "$NODE1_MINA_PRIV_KEY"
EOF

cat >> ~/.pulsar-node2/config/app.toml << EOF

[vote_extension]
priv_key = "$NODE2_MINA_PRIV_KEY"
EOF

echo ""
echo "==> Done! Start the nodes with:"
echo ""
echo "  Terminal 1:"
echo "  $BINARY start --home $NODE1_HOME"
echo ""
echo "  Terminal 2:"
echo "  $BINARY start --home $NODE2_HOME"
echo ""
echo "  Verify 2 validators after starting:"
echo "  curl -s http://localhost:26657/validators | python3 -m json.tool | grep total"