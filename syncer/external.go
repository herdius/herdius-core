package sync

import (
	"math/big"
	"time"

	"github.com/herdius/herdius-core/p2p/log"
	external "github.com/herdius/herdius-core/storage/exbalance"
	"github.com/herdius/herdius-core/storage/state/statedb"
	"github.com/herdius/herdius-core/symbol"
)

const (
	rpcTimeout  = time.Second * 2
	httpTimeout = time.Second * 2
)

// ExternalSyncer syncs all asset external accounts
type ExternalSyncer struct {
	LastExtBalance map[string]*big.Int
	ExtBalance     map[string]*big.Int
	BlockHeight    map[string]*big.Int
	Nonce          map[string]uint64
	RPC            string
	Account        statedb.Account
	Storage        external.BalanceStorage
	addressError   map[string]bool
	assetSymbol    string
}

func newExternalSyncer(assetSymbol string) *ExternalSyncer {
	e := &ExternalSyncer{}
	e.ExtBalance = make(map[string]*big.Int)
	e.LastExtBalance = make(map[string]*big.Int)
	e.BlockHeight = make(map[string]*big.Int)
	e.Nonce = make(map[string]uint64)
	e.addressError = make(map[string]bool)
	e.assetSymbol = assetSymbol

	return e
}

func (es *ExternalSyncer) isHBTC() bool {
	return es.assetSymbol == symbol.HBTC
}

func (es *ExternalSyncer) update(address string) {
	assetSymbol := es.assetSymbol
	assetAccount := es.Account.EBalances[es.assetSymbol][address]

	herEthBalance := *big.NewInt(int64(0))
	storageKey := assetSymbol + "-" + assetAccount.Address
	if es.ExtBalance[assetAccount.Address] == nil {
		es.ExtBalance[assetAccount.Address] = big.NewInt(0)
	}
	//fmt.Println("storageKey::::", storageKey)
	if last, ok := es.Storage.Get(es.Account.Address); ok {

		// last-balance < External-ETH
		// Balance of ETH in H = Balance of ETH in H + ( Current_External_Bal - last_External_Bal_In_Cache)
		if lastExtBalance, ok := last.LastExtBalance[storageKey]; ok && lastExtBalance != nil {
			if lastExtBalance.Cmp(es.ExtBalance[assetAccount.Address]) < 0 {
				//log.Debug().Msgf("lastExtBalance.Cmp(es.ExtBalance[%s])", assetAccount.Address)

				herEthBalance.Sub(es.ExtBalance[assetAccount.Address], lastExtBalance)

				assetAccount.Balance += herEthBalance.Uint64()
				if es.BlockHeight[assetAccount.Address] != nil {
					assetAccount.LastBlockHeight = es.BlockHeight[assetAccount.Address].Uint64()
				}
				assetAccount.Nonce = es.Nonce[assetAccount.Address]
				es.Account.EBalances[assetSymbol][assetAccount.Address] = assetAccount

				last = last.UpdateLastExtBalanceByKey(storageKey, es.ExtBalance[assetAccount.Address])
				last = last.UpdateCurrentExtBalanceByKey(storageKey, es.ExtBalance[assetAccount.Address])
				last = last.UpdateIsFirstEntryByKey(storageKey, false)
				last = last.UpdateIsNewAmountUpdateByKey(storageKey, true)
				last = last.UpdateAccount(es.Account)
				es.Storage.Set(es.Account.Address, last)
				log.Debug().Msgf("New account balance after external balance credit: %v\n", last)
			}

			// last-balance < External-ETH
			// Balance of ETH in H1 	= Balance of ETH in H - ( last_External_Bal_In_Cache - Current_External_Bal )
			if lastExtBalance.Cmp(es.ExtBalance[assetAccount.Address]) > 0 {
				//	log.Debug().Msg("lastExtBalance.Cmp(es.ExtBalance) ============")
				herEthBalance.Sub(lastExtBalance, es.ExtBalance[assetAccount.Address])
				if assetAccount.Balance >= herEthBalance.Uint64() {
					assetAccount.Balance -= herEthBalance.Uint64()
					if es.BlockHeight[assetAccount.Address] != nil {
						assetAccount.LastBlockHeight = es.BlockHeight[assetAccount.Address].Uint64()
					}
					assetAccount.Nonce = es.Nonce[assetAccount.Address]
					es.Account.EBalances[assetSymbol][assetAccount.Address] = assetAccount

					last = last.UpdateLastExtBalanceByKey(storageKey, es.ExtBalance[assetAccount.Address])
					last = last.UpdateCurrentExtBalanceByKey(storageKey, es.ExtBalance[assetAccount.Address])
					last = last.UpdateIsFirstEntryByKey(storageKey, false)
					last = last.UpdateIsNewAmountUpdateByKey(storageKey, true)
					last = last.UpdateAccount(es.Account)
					es.Storage.Set(es.Account.Address, last)
				}
				//log.Debug().Msgf("New account balance after external balance debit: %v\n", last)
			}

			return
		}

		//log.Info().Msgf("Initialise external balance in cache: %v\n", last)
		if es.ExtBalance[assetAccount.Address] == nil {
			es.ExtBalance[assetAccount.Address] = big.NewInt(0)
		}
		if es.BlockHeight[assetAccount.Address] == nil {
			es.BlockHeight[assetAccount.Address] = big.NewInt(0)
		}
		last = last.UpdateLastExtBalanceByKey(storageKey, es.ExtBalance[assetAccount.Address])
		last = last.UpdateCurrentExtBalanceByKey(storageKey, es.ExtBalance[assetAccount.Address])
		last = last.UpdateIsFirstEntryByKey(storageKey, true)
		last = last.UpdateIsNewAmountUpdateByKey(storageKey, false)
		assetAccount.UpdateBalance(es.ExtBalance[assetAccount.Address].Uint64())
		assetAccount.UpdateBlockHeight(es.BlockHeight[assetAccount.Address].Uint64())
		assetAccount.UpdateNonce(es.Nonce[assetAccount.Address])
		es.Account.EBalances[assetSymbol][assetAccount.Address] = assetAccount
		last = last.UpdateAccount(es.Account)
		es.Storage.Set(es.Account.Address, last)
		return
	}

	log.Info().Msg("Initialise account in cache.")
	balance := es.ExtBalance[assetAccount.Address]
	blockHeight := es.BlockHeight[assetAccount.Address]
	lastbalances := make(map[string]*big.Int)
	lastbalances[storageKey] = es.ExtBalance[assetAccount.Address]

	currentbalances := make(map[string]*big.Int)
	currentbalances[storageKey] = es.ExtBalance[assetAccount.Address]
	if balance == nil {
		lastbalances[storageKey] = big.NewInt(0)
		currentbalances[storageKey] = big.NewInt(0)
	}
	isFirstEntry := make(map[string]bool)
	isFirstEntry[storageKey] = true
	isNewAmountUpdate := make(map[string]bool)
	isNewAmountUpdate[storageKey] = false
	if balance != nil {
		assetAccount.UpdateBalance(balance.Uint64())
	}
	if blockHeight != nil {
		assetAccount.UpdateBlockHeight(blockHeight.Uint64())
	}
	assetAccount.UpdateNonce(es.Nonce[assetAccount.Address])

	es.Account.EBalances[assetSymbol][assetAccount.Address] = assetAccount
	val := external.AccountCache{
		Account:           es.Account,
		LastExtBalance:    lastbalances,
		CurrentExtBalance: currentbalances,
		IsFirstEntry:      isFirstEntry,
		IsNewAmountUpdate: isNewAmountUpdate,
	}
	es.Storage.Set(es.Account.Address, val)
}
