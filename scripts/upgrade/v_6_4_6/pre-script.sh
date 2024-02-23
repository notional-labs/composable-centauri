echo "********** Running Pre-Scripts **********"

BINARY=$1 
DENOM=${2:-pica}
CHAIN_DIR=$(pwd)/mytestnet

KEY="test0"
KEY1="test1"
KEY2="test2"

WALLET_1=$($BINARY keys show $KEY1 -a --keyring-backend test --home $CHAIN_DIR)  
echo "wallet 1: $WALLET_1"


DEFAULT_GAS_FLAG="--gas 3000000 --gas-prices 0.025$DENOM --gas-adjustment 1.5"
DEFAULT_ENV_FLAG="--keyring-backend test --chain-id localpica --home $CHAIN_DIR"

echo "binary value: $BINARY"
echo "pwd: $(pwd)"
COUNTER_CONTRACT_DIR=$(pwd)/scripts/upgrade/contracts/counter.wasm



############ Settingup WASM environment ############
### Create a counter contract, then increment the counter to 1 ####
## Deploy the counter contract 
TX_HASH=$($BINARY tx wasm store $COUNTER_CONTRACT_DIR --from $KEY1 $DEFAULT_ENV_FLAG $DEFAULT_GAS_FLAG -y -o json | jq -r '.txhash')

## Get CODE ID
sleep 1
CODE_ID=$($BINARY query tx $TX_HASH -o json | jq -r '.logs[0].events[1].attributes[1].value')
echo "code id: $CODE_ID"

## Get contract address
# NOTE: CAN USE https://github.com/CosmWasm/wasmd/blob/9e44af168570391b0b69822952f206d35320d473/contrib/local/02-contracts.sh#L38 instantiate2 to predict address
RANDOM_HASH=$(hexdump -vn16 -e'4/4 "%08X" 1 "\n"' /dev/urandom)
TX_HASH=$($BINARY tx wasm instantiate2 $CODE_ID '{"count": 0}' $RANDOM_HASH --no-admin --label="Label with $RANDOM_HASH" --from $KEY1 $DEFAULT_ENV_FLAG $DEFAULT_GAS_FLAG -y -o json | jq -r '.txhash')

sleep 1
CONTRACT_ADDRESS=$($BINARY query tx $TX_HASH -o json | jq -r '.logs[0].events[1].attributes[0].value')
echo "Contract address: $CONTRACT_ADDRESS"

## Execute the contract, increment counter to 1
$BINARY tx wasm execute $CONTRACT_ADDRESS '{"increment":{}}' --from $KEY1 $DEFAULT_ENV_FLAG $DEFAULT_GAS_FLAG -y -o json > /dev/null

## assert counter value to be 1
sleep 1
COUNTER_VALUE=$($BINARY query wasm contract-state smart $CONTRACT_ADDRESS '{"get_count":{"addr": "'"$WALLET_1"'"}}' -o json | jq -r '.data.count')
if [ "$COUNTER_VALUE" -ne 1 ]; then
    echo "Assertion failed: Expected counter value to be 1, got $COUNTER_VALUE"
    exit 1
fi
echo "Assertion passed: Counter value is 1 as expected"

export CONTRACT_ADDRESS=$CONTRACT_ADDRESS
