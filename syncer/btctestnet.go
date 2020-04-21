package sync

import (
	"errors"
	"math/big"
	"strings"

	blockcypher "github.com/blockcypher/gobcy"

	"github.com/herdius/herdius-core/p2p/log"
	external "github.com/herdius/herdius-core/storage/exbalance"
	"github.com/herdius/herdius-core/storage/state/statedb"
	"github.com/herdius/herdius-core/symbol"
)

// BTCTestNetSyncer syncs all external BTC accounts in btctestnet.
type BTCTestNetSyncer struct {
	LastExtBalance map[string]*big.Int
	ExtBalance     map[string]*big.Int
	BlockHeight    map[string]*big.Int
	Nonce          map[string]uint64
	RPC            string
	Account        statedb.Account
	Storage        external.BalanceStorage
	addressError   map[string]bool
}

func newBTCTestNetSyncer() *BTCTestNetSyncer {
	b := &BTCTestNetSyncer{}
	b.ExtBalance = make(map[string]*big.Int)
	b.LastExtBalance = make(map[string]*big.Int)
	b.BlockHeight = make(map[string]*big.Int)
	b.Nonce = make(map[string]uint64)
	b.addressError = make(map[string]bool)

	return b
}

// GetExtBalance ...
func (btc *BTCTestNetSyncer) GetExtBalance() error {

	btcAccount, ok := btc.Account.EBalances[symbol.BTC]
	if !ok {
		return errors.New("BTC account does not exists")
	}
	//btcCypher := blockcypher.API{Token: "d3b5ec2c94cc4ba183f02aca9d8729da", Coin: "btc", Chain: "test3"}
	btcCypher := blockcypher.API{Token: "1b45141eefcd4814b8a1ff746f651854", Coin: "btc", Chain: "test3"}
	for _, ba := range btcAccount {
		if strings.HasPrefix(ba.Address, "1") || strings.HasPrefix(ba.Address, "3") {
			btc.addressError[ba.Address] = true
			log.Warn().Msgf("Address %s is a main network, not btc testnet, do not sync", ba.Address)
			continue
		}
		addr, err := btcCypher.GetAddrFull(ba.Address, nil)
		if err != nil {
			log.Error().Err(err).Msg("Error getting BTC address in btctestnet")
			btc.addressError[ba.Address] = true
			// This relies on how blockcypher handles error
			// See https://github.com/blockcypher/gobcy/blob/6eace16b4b81ea8dbdcde5417d6b1cc3828a3d1f/gobcy.go#L118
			if strings.HasPrefix(err.Error(), "HTTP 429") {
				log.Error().Msg("Rate limit reached, stop sync btctestnet")
				return err
			}
			continue
		}
		if len(addr.TXs) > 0 {
			btc.BlockHeight[ba.Address] = big.NewInt(int64(addr.TXs[0].BlockHeight))
			btc.Nonce[ba.Address] = uint64(len(addr.TXs))
			btc.ExtBalance[ba.Address] = big.NewInt(int64(addr.Balance))
			btc.addressError[ba.Address] = false
		}
	}

	return nil

}

// Update updates accounts in cache as and when external balances
// external chains are updated.
func (btc *BTCTestNetSyncer) Update() {
	assetSymbol := symbol.BTC
	for _, btcAccount := range btc.Account.EBalances[assetSymbol] {
		if btc.addressError[btcAccount.Address] {
			//log.Warn().Msgf("Account info is not available at this moment, skip sync: %s", btcAccount.Address)
			continue
		}
		herEthBalance := *big.NewInt(int64(0))
		storageKey := assetSymbol + "-" + btcAccount.Address
		if last, ok := btc.Storage.Get(btc.Account.Address); ok {
			// last-balance < External-ETH
			// Balance of ETH in H = Balance of ETH in H + ( Current_External_Bal - last_External_Bal_In_Cache)
			if lastExtBalance, ok := last.LastExtBalance[storageKey]; ok && lastExtBalance != nil {
				// We need to guard here because buggy code before causing ext balance
				// for given btc account set to nil.
				if btc.ExtBalance[btcAccount.Address] == nil {
					btc.ExtBalance[btcAccount.Address] = big.NewInt(0)
				}
				if lastExtBalance.Cmp(btc.ExtBalance[btcAccount.Address]) < 0 {
					log.Debug().Msgf("lastExtBalance.Cmp(btc.ExtBalance[%s])", btcAccount.Address)

					herEthBalance.Sub(btc.ExtBalance[btcAccount.Address], lastExtBalance)

					btcAccount.Balance += herEthBalance.Uint64()
					btcAccount.LastBlockHeight = btc.BlockHeight[btcAccount.Address].Uint64()
					btcAccount.Nonce = btc.Nonce[btcAccount.Address]
					btc.Account.EBalances[assetSymbol][btcAccount.Address] = btcAccount

					last = last.UpdateLastExtBalanceByKey(storageKey, btc.ExtBalance[btcAccount.Address])
					last = last.UpdateCurrentExtBalanceByKey(storageKey, btc.ExtBalance[btcAccount.Address])
					last = last.UpdateIsFirstEntryByKey(storageKey, false)
					last = last.UpdateIsNewAmountUpdateByKey(storageKey, true)
					last = last.UpdateAccount(btc.Account)
					btc.Storage.Set(btc.Account.Address, last)

					log.Debug().Msgf("New account balance after external balance credit: %v\n", last)
				}

				// last-balance < External-ETH
				// Balance of ETH in H1 	= Balance of ETH in H - ( last_External_Bal_In_Cache - Current_External_Bal )
				if lastExtBalance.Cmp(btc.ExtBalance[btcAccount.Address]) > 0 {
					log.Debug().Msg("lastExtBalance.Cmp(btc.ExtBalance) ============")

					herEthBalance.Sub(lastExtBalance, btc.ExtBalance[btcAccount.Address])
					if btcAccount.Balance >= herEthBalance.Uint64() {
						btcAccount.Balance -= herEthBalance.Uint64()
						btcAccount.LastBlockHeight = btc.BlockHeight[btcAccount.Address].Uint64()
						btcAccount.Nonce = btc.Nonce[btcAccount.Address]
						btc.Account.EBalances[assetSymbol][btcAccount.Address] = btcAccount

						last = last.UpdateLastExtBalanceByKey(storageKey, btc.ExtBalance[btcAccount.Address])
						last = last.UpdateCurrentExtBalanceByKey(storageKey, btc.ExtBalance[btcAccount.Address])
						last = last.UpdateIsFirstEntryByKey(storageKey, false)
						last = last.UpdateIsNewAmountUpdateByKey(storageKey, true)
						last = last.UpdateAccount(btc.Account)
						btc.Storage.Set(btc.Account.Address, last)
					}
					//log.Debug().Msgf("New account balance after external balance debit: %v\n", last)
				}
				continue
			}

			log.Printf("Initialise external balance in cache: %v\n", last)
			if btc.ExtBalance[btcAccount.Address] == nil {
				btc.ExtBalance[btcAccount.Address] = big.NewInt(0)
			}
			if btc.BlockHeight[btcAccount.Address] == nil {
				btc.BlockHeight[btcAccount.Address] = big.NewInt(0)
			}
			last = last.UpdateLastExtBalanceByKey(storageKey, btc.ExtBalance[btcAccount.Address])
			last = last.UpdateCurrentExtBalanceByKey(storageKey, btc.ExtBalance[btcAccount.Address])
			last = last.UpdateIsFirstEntryByKey(storageKey, true)
			last = last.UpdateIsNewAmountUpdateByKey(storageKey, false)
			btcAccount.UpdateBalance(btc.ExtBalance[btcAccount.Address].Uint64())
			btcAccount.UpdateBlockHeight(btc.BlockHeight[btcAccount.Address].Uint64())
			btcAccount.UpdateNonce(btc.Nonce[btcAccount.Address])
			btc.Account.EBalances[assetSymbol][btcAccount.Address] = btcAccount
			last = last.UpdateAccount(btc.Account)
			btc.Storage.Set(btc.Account.Address, last)
			continue
		}

		log.Printf("Initialise account in cache.")
		balance := btc.ExtBalance[btcAccount.Address]
		blockHeight := btc.BlockHeight[btcAccount.Address]
		lastbalances := make(map[string]*big.Int)
		lastbalances[storageKey] = btc.ExtBalance[btcAccount.Address]

		currentbalances := make(map[string]*big.Int)
		currentbalances[storageKey] = btc.ExtBalance[btcAccount.Address]
		if balance == nil {
			lastbalances[storageKey] = big.NewInt(0)
			currentbalances[storageKey] = big.NewInt(0)
		}
		isFirstEntry := make(map[string]bool)
		isFirstEntry[storageKey] = true
		isNewAmountUpdate := make(map[string]bool)
		isNewAmountUpdate[storageKey] = false
		if balance != nil {
			btcAccount.UpdateBalance(balance.Uint64())
		}
		if blockHeight != nil {
			btcAccount.UpdateBlockHeight(blockHeight.Uint64())
		}
		btcAccount.UpdateNonce(btc.Nonce[btcAccount.Address])

		btc.Account.EBalances[assetSymbol][btcAccount.Address] = btcAccount
		val := external.AccountCache{
			Account: btc.Account, LastExtBalance: lastbalances, CurrentExtBalance: currentbalances, IsFirstEntry: isFirstEntry, IsNewAmountUpdate: isNewAmountUpdate,
		}
		btc.Storage.Set(btc.Account.Address, val)
	}
}
