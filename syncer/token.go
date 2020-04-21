package sync

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/herdius/herdius-core/p2p/log"
	external "github.com/herdius/herdius-core/storage/exbalance"
	"github.com/herdius/herdius-core/storage/state/statedb"
	"github.com/herdius/herdius-core/syncer/contract"
)

type HERToken struct {
	LastExtBalance       *big.Int
	ExtBalance           *big.Int
	Account              statedb.Account
	BlockHeight          *big.Int
	Nonce                uint64
	Storage              external.BalanceStorage
	TokenContractAddress string
	TokenSymbol          string
	RPC                  string
}

//GetExtBalance Gets Asset balance from main chain
func (her *HERToken) GetExtBalance() error {
	var (
		latestBlockNumber *big.Int
		nonce             uint64
		err               error
	)
	client, err := ethclient.Dial(her.RPC)
	if err != nil {
		log.Error().Err(err).Msg("Error connecting ETH RPC")
		return err
	}
	tokenAddress := common.HexToAddress(her.TokenContractAddress)
	address := common.HexToAddress(her.Account.Erc20Address)

	// Get latest block number
	latestBlockNumber, err = her.getLatestBlockNumber(client)
	if err != nil {
		log.Error().Err(err).Msg("Error getting TOKEN Latest block from RPC")
		return err
	}

	//Get nonce
	nonce, err = her.getNonce(client, address, latestBlockNumber)
	if err != nil {
		log.Error().Err(err).Msg("Error getting TOKEN Account nonce from RPC")
		return err
	}

	instance, err := contract.NewToken(tokenAddress, client)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	bal, err := instance.BalanceOf(&bind.CallOpts{BlockNumber: latestBlockNumber, Context: ctx}, address)
	if err != nil {
		return err
	}

	her.ExtBalance = bal
	her.BlockHeight = latestBlockNumber
	her.Nonce = nonce

	return nil
}

//Update Updates balance of asset in cache
func (her *HERToken) Update() {
	herBalance := *big.NewInt(int64(0))
	last, ok := her.Storage.Get(her.Account.Address)

	if ok {
		//last-balance < External-ETH
		//Balance of ETH in H = Balance of ETH in H + ( Current_External_Bal - last_External_Bal_In_Cache)
		lastExtHERBalance := last.LastExtHERBalance
		if lastExtHERBalance != nil {
			if lastExtHERBalance.Cmp(her.ExtBalance) < 0 {
				herBalance.Sub(her.ExtBalance, lastExtHERBalance)
				her.Account.Balance += herBalance.Uint64()
				her.Account.ExternalNonce = her.Nonce
				her.Account.LastBlockHeight = her.BlockHeight.Uint64()

				last = last.UpdateAccount(her.Account)
				last = last.UpdateLastExtHERBalance(her.ExtBalance)
				last = last.UpdateCurrentExtHERBalance(her.ExtBalance)
				last = last.UpdateIsNewHERAmountUpdate(true)
				last = last.UpdateIsFirstHER(false)

				log.Debug().Msgf("New account balance after external balance credit: %v\n", last)
				her.Storage.Set(her.Account.Address, last)
				return

			}

			//last-balance < External-ETH
			//Balance of ETH in H1 	= Balance of ETH in H - ( last_External_Bal_In_Cache - Current_External_Bal )
			if lastExtHERBalance.Cmp(her.ExtBalance) > 0 {
				herBalance.Sub(lastExtHERBalance, her.ExtBalance)
				her.Account.Balance -= herBalance.Uint64()
				her.Account.ExternalNonce = her.Nonce
				her.Account.LastBlockHeight = her.BlockHeight.Uint64()
				last = last.UpdateAccount(her.Account)
				last = last.UpdateLastExtHERBalance(her.ExtBalance)
				last = last.UpdateCurrentExtHERBalance(her.ExtBalance)
				last = last.UpdateIsNewHERAmountUpdate(true)
				last = last.UpdateIsFirstHER(false)

				//log.Debug().Msgf("New account balance after external balance debit: %v\n", last)
				her.Storage.Set(her.Account.Address, last)
				return
			}
		} else {

			her.Account.Balance = her.ExtBalance.Uint64()
			her.Account.ExternalNonce = her.Nonce
			her.Account.LastBlockHeight = her.BlockHeight.Uint64()

			last = last.UpdateAccount(her.Account)
			last = last.UpdateIsFirstHER(true)
			last = last.UpdateLastExtHERBalance(her.ExtBalance)
			last = last.UpdateCurrentExtHERBalance(her.ExtBalance)

			her.Storage.Set(her.Account.Address, last)

		}

	} else {
		if len(her.Account.Erc20Address) > 0 {
			val := external.AccountCache{
				Account: her.Account,
			}
			her.Storage.Set(her.Account.Address, val)
		}
	}

}

func (her *HERToken) getLatestBlockNumber(client *ethclient.Client) (*big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	return header.Number, nil
}

func (her *HERToken) getNonce(client *ethclient.Client, account common.Address, block *big.Int) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	nonce, err := client.NonceAt(ctx, account, block)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}
