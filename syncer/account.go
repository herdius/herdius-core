package sync

import (
	"fmt"
	"os"
	"strings"
	stdSync "sync"

	"github.com/ethereum/go-ethereum/common"
	ethtrie "github.com/ethereum/go-ethereum/trie"
	"github.com/spf13/viper"

	"github.com/herdius/herdius-core/blockchain"
	"github.com/herdius/herdius-core/p2p/log"
	external "github.com/herdius/herdius-core/storage/exbalance"
	"github.com/herdius/herdius-core/storage/state/statedb"
	"github.com/herdius/herdius-core/symbol"
)

type apiEndponts struct {
	btcRPC   string
	ethRPC   string
	tezosRPC string
	ltcRPC   string
	bnbRPC   string
	eosRPC   string

	hbtcRPC string
	hbnbRPC string
	hltcRPC string
	hxtzRPC string
	heosRPC string

	herTokenAddress string
	daiTokenAddress string
}

// DoSyncAllAccounts syncs all assets of available accounts.
func DoSyncAllAccounts(exBal external.BalanceStorage, env string, stopCh chan struct{}) {
	var rpc apiEndponts
	viper.SetConfigName("config")   // Config file name without extension
	viper.AddConfigPath("./config") // Path to config file
	err := viper.ReadInConfig()
	if err != nil {
		log.Error().Err(err).Msg("failed to read config file")
		return
	}

	rpc.ethRPC = viper.GetString(env + ".ethrpc")
	rpc.herTokenAddress = viper.GetString(env + ".hercontractaddress")
	rpc.daiTokenAddress = viper.GetString(env + ".daicontractaddress")
	rpc.btcRPC = viper.GetString(env + ".blockchaininforpc")
	rpc.tezosRPC = viper.GetString(env + ".tezosrpc")
	rpc.ltcRPC = viper.GetString(env + ".ltcrpc")
	rpc.bnbRPC = viper.GetString(env + ".bnbrpc")
	rpc.eosRPC = viper.GetString(env + ".eosrpc")
	rpc.hbtcRPC = fmt.Sprintf(viper.GetString(env+".htokenrpc"), strings.ToLower(symbol.HBTC))
	rpc.hbnbRPC = fmt.Sprintf(viper.GetString(env+".htokenrpc"), strings.ToLower(symbol.HBNB))
	rpc.hltcRPC = fmt.Sprintf(viper.GetString(env+".htokenrpc"), strings.ToLower(symbol.HLTC))
	rpc.hxtzRPC = fmt.Sprintf(viper.GetString(env+".htokenrpc"), strings.ToLower(symbol.HXTZ))
	rpc.heosRPC = fmt.Sprintf(viper.GetString(env+".htokenrpc"), strings.ToLower(symbol.HXTZ))

	if strings.Index(rpc.ethRPC, ".infura.io") > -1 {
		rpc.ethRPC += os.Getenv("INFURAID")
	}

	for {
		select {
		case <-stopCh:
			return
		default:
			sync(exBal, rpc)
		}
	}
}

func sync(exBal external.BalanceStorage, rpc apiEndponts) {
	blockchainSvc := &blockchain.Service{}
	lastBlock := blockchainSvc.GetLastBlock()
	stateRoot := lastBlock.GetHeader().GetStateRoot()

	stateTrie, err := ethtrie.New(common.BytesToHash(stateRoot), statedb.GetDB())
	if err != nil {
		log.Error().Err(err).Msg("failed to retrieve the state trie.")
		return
	}
	it := ethtrie.NewIterator(stateTrie.NodeIterator(nil))

	log.Debug().Msg("Sync account start")
	var wg stdSync.WaitGroup
	// TODO: make this configuration
	semaphore := make(chan struct{}, 50)
	for it.Next() {
		var senderAccount statedb.Account
		senderAddressBytes := it.Key
		senderActbz, err := stateTrie.TryGet(senderAddressBytes)
		if err != nil {
			log.Error().Err(err).Msg("failed to retrieve account detail")
			continue
		}

		if len(senderActbz) > 0 {
			err = cdc.UnmarshalJSON(senderActbz, &senderAccount)
			if err != nil {
				log.Warn().Err(err).Msg("failed to Unmarshal account")
				// Try unmarshal to old account struct
				var oldAccount statedb.OldAccount
				if err := cdc.UnmarshalJSON(senderActbz, &oldAccount); err != nil {
					log.Error().Err(err).Msg("failed to Unmarshal old account")
					continue
				}
				log.Debug().Msg("Sync old account before supporting multiple ebalances added.")
				senderAccount.Address = oldAccount.Address
				senderAccount.AddressHash = oldAccount.AddressHash
				senderAccount.Balance = oldAccount.Balance
				senderAccount.Erc20Address = oldAccount.Erc20Address
				senderAccount.ExternalNonce = oldAccount.ExternalNonce
				senderAccount.LastBlockHeight = oldAccount.LastBlockHeight
				senderAccount.Nonce = oldAccount.Nonce
				senderAccount.PublicKey = oldAccount.PublicKey
				senderAccount.StateRoot = oldAccount.StateRoot
				senderAccount.FirstExternalAddress = make(map[string]string)
				senderAccount.EBalances = make(map[string]map[string]statedb.EBalance)
				for asset, assetAccount := range oldAccount.EBalances {
					senderAccount.EBalances[asset] = make(map[string]statedb.EBalance)
					senderAccount.EBalances[asset][assetAccount.Address] = assetAccount
					senderAccount.FirstExternalAddress = make(map[string]string)
					senderAccount.FirstExternalAddress[asset] = assetAccount.Address
				}
			}
		}
		var syncers []Syncer

		// ETH syncer
		ethSyncer := newEthSyncer()
		ethSyncer.RPC = rpc.ethRPC
		ethSyncer.syncer.Account = senderAccount
		ethSyncer.syncer.Storage = exBal
		syncers = append(syncers, ethSyncer)

		// BTC syncer
		btcSyncer := newBTCSyncer()
		btcSyncer.RPC = rpc.btcRPC
		btcSyncer.syncer.Account = senderAccount
		btcSyncer.syncer.Storage = exBal
		syncers = append(syncers, btcSyncer)

		// HBTC syncer
		hbtcSyncer := newHBTCSyncer()
		hbtcSyncer.RPC = rpc.hbtcRPC
		hbtcSyncer.syncer.Account = senderAccount
		hbtcSyncer.syncer.Storage = exBal
		syncers = append(syncers, hbtcSyncer)

		// HBTC testnetsyncer
		hbtctestSyncer := newBTCTestNetSyncer()
		hbtctestSyncer.Account = senderAccount
		hbtctestSyncer.Storage = exBal
		syncers = append(syncers, hbtctestSyncer)

		// HERDIUS syncer
		syncers = append(syncers, &HERToken{Account: senderAccount, Storage: exBal, RPC: rpc.ethRPC, TokenContractAddress: rpc.herTokenAddress})

		// TEZOS syncer
		tezosSyncer := newTezosSyncer()
		tezosSyncer.RPC = rpc.tezosRPC
		tezosSyncer.syncer.Account = senderAccount
		tezosSyncer.syncer.Storage = exBal
		syncers = append(syncers, tezosSyncer)

		// DAI syncer
		daiSyncer := newDAISyncer()
		daiSyncer.RPC = rpc.ethRPC
		daiSyncer.TokenContractAddress = rpc.daiTokenAddress
		daiSyncer.syncer.Account = senderAccount
		daiSyncer.syncer.Storage = exBal
		syncers = append(syncers, daiSyncer)

		// LTC syncer
		ltcSyncer := newLTCSyncer()
		ltcSyncer.RPC = rpc.ltcRPC
		ltcSyncer.syncer.Account = senderAccount
		ltcSyncer.syncer.Storage = exBal
		syncers = append(syncers, ltcSyncer)

		// BNB syncer
		bnbSyncer := newBNBSyncer()
		bnbSyncer.RPC = rpc.bnbRPC
		bnbSyncer.syncer.Account = senderAccount
		bnbSyncer.syncer.Storage = exBal
		syncers = append(syncers, bnbSyncer)

		// EOS syncer
		eosSyncer := newEOSSyncer()
		eosSyncer.RPC = rpc.eosRPC
		eosSyncer.syncer.Account = senderAccount
		eosSyncer.syncer.Storage = exBal
		syncers = append(syncers, eosSyncer)

		// HLTC syncer
		hltcSyncer := newHLTCSyncer()
		hltcSyncer.RPC = rpc.hltcRPC
		hltcSyncer.syncer.Account = senderAccount
		hltcSyncer.syncer.Storage = exBal
		syncers = append(syncers, hltcSyncer)

		// HBNB syncer
		hbnbSyncer := newHBNBSyncer()
		hbnbSyncer.RPC = rpc.hbnbRPC
		hbnbSyncer.syncer.Account = senderAccount
		hbnbSyncer.syncer.Storage = exBal
		syncers = append(syncers, hbnbSyncer)

		// HXTZ syncer
		hxtzSyncer := newHXTZSyncer()
		hxtzSyncer.RPC = rpc.hxtzRPC
		hxtzSyncer.syncer.Account = senderAccount
		hxtzSyncer.syncer.Storage = exBal
		syncers = append(syncers, hxtzSyncer)

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire Lock
			semaphore <- struct{}{}
			defer func() {
				// Release Lock
				<-semaphore
			}()

			for _, asset := range syncers {
				// Dont update account if no new value received from respective api calls
				if asset.GetExtBalance() == nil {
					asset.Update()
				}

			}
		}()
	}
	wg.Wait()
	log.Debug().Msg("Sync account end")
}
