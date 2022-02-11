#!/bin/sh

set -o errexit -o nounset

CHAINID="test"

# Build genesis file incl account for passed address
coins="1000000000000000uceles"
celestia-appd init $CHAINID --chain-id $CHAINID 
celestia-appd keys add validator --keyring-backend="test"
# this won't work because the some proto types are decalared twice and the logs output to stdout (dependency hell involving iavl)
celestia-appd add-genesis-account $(celestia-appd keys show validator -a --keyring-backend="test") $coins
celestia-appd gentx validator 5000000000uceles --keyring-backend="test" --chain-id $CHAINID
celestia-appd collect-gentxs

# Set proper defaults and change ports
# might have to use the -i flag 
sed 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:26657"#g' ~/.celestia-app/config/config.toml
sed 's/timeout_commit = "5s"/timeout_commit = "1s"/g' ~/.celestia-app/config/config.toml
sed 's/timeout_propose = "3s"/timeout_propose = "1s"/g' ~/.celestia-app/config/config.toml
sed 's/index_all_keys = false/index_all_keys = true/g' ~/.celestia-app/config/config.toml

# Start the celestia-app
celestia-appd start
