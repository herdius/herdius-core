package sync

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/herdius/herdius-core/p2p/log"
	"github.com/herdius/herdius-core/symbol"
	"github.com/spf13/viper"
	"github.com/tendermint/go-amino"
)

var cdc = amino.NewCodec()

// EthSyncer syncs all ETH external accounts
type EthSyncer struct {
	RPC    string
	syncer *ExternalSyncer
}

func newEthSyncer() *EthSyncer {
	e := &EthSyncer{}
	e.syncer = newExternalSyncer(symbol.ETH)

	return e
}

// GetExtBalance ...
func (es *EthSyncer) GetExtBalance() error {
	// If ETH account exists
	ethAccount, ok := es.syncer.Account.EBalances[es.syncer.assetSymbol]
	if !ok {
		return errors.New("ETH account does not exists")
	}

	for _, ea := range ethAccount {
		var (
			balance, latestBlockNumber *big.Int
			nonce                      uint64
			err                        error
		)
		client, err := ethclient.Dial(es.RPC)
		if err != nil {
			log.Error().Msgf("Error connecting ETH RPC: %v", err)
			es.syncer.addressError[ea.Address] = true
			continue
		}

		account := common.HexToAddress(ea.Address)

		// Get latest block number
		latestBlockNumber, err = es.getLatestBlockNumber(client)
		if err != nil {
			log.Error().Msgf("Error getting ETH Latest block from RPC: %v", err)
			es.syncer.addressError[ea.Address] = true
			continue
		}

		// Get nonce
		nonce, err = es.getNonce(client, account, latestBlockNumber)
		if err != nil {
			log.Error().Msgf("Error getting ETH Account nonce from RPC: %v", err)
			es.syncer.addressError[ea.Address] = true
			continue
		}

		// Get balance
		balance, err = es.getBalance(ea.Address)
		if err != nil {
			log.Error().Msgf("Error getting ETH Balance from RPC: %v", err)
			es.syncer.addressError[ea.Address] = true
			continue
		}

		es.syncer.ExtBalance[ea.Address] = balance
		es.syncer.BlockHeight[ea.Address] = latestBlockNumber
		es.syncer.Nonce[ea.Address] = nonce
		es.syncer.addressError[ea.Address] = false
	}

	return nil
}

// Update updates accounts in cache as and when external balances
// external chains are updated.
func (es *EthSyncer) Update() {
	for _, assetAccount := range es.syncer.Account.EBalances[es.syncer.assetSymbol] {
		if es.syncer.addressError[assetAccount.Address] {
			//log.Warn().Msg("ETH Account info is not available at this moment, skip sync: " + assetAccount.Address)
			continue
		}
		es.syncer.update(assetAccount.Address)
	}
}

func (es *EthSyncer) getLatestBlockNumber(client *ethclient.Client) (*big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	return header.Number, nil
}

func (es *EthSyncer) getNonce(client *ethclient.Client, account common.Address, block *big.Int) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	nonce, err := client.NonceAt(ctx, account, block)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

func (es *EthSyncer) getBalance(address string) (*big.Int, error) {
	httpClient := newHTTPClient()
	balanceurl := viper.GetString("dev.balanceapi") + address
	resp, err := httpClient.Get(balanceurl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type AddressBalance struct {
		Status  bool     `json:"status"`
		Message string   `json:"message"`
		Data    *big.Int `json:"data"`
	}

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	balanceInfo := new(AddressBalance)
	json.Unmarshal(bodyBytes, balanceInfo)

	if !balanceInfo.Status {
		return nil, err
	}
	return balanceInfo.Data, nil
}
