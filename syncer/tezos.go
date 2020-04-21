package sync

import (
	"errors"
	"math/big"

	"github.com/herdius/herdius-core/p2p/log"
	"github.com/herdius/herdius-core/symbol"

	goTezos "github.com/DefinitelyNotAGoat/go-tezos"
	goTezosAccount "github.com/DefinitelyNotAGoat/go-tezos/account"
)

// TezosSyncer syncs all XTZ external accounts
type TezosSyncer struct {
	RPC    string
	syncer *ExternalSyncer
}

func newTezosSyncer() *TezosSyncer {
	t := &TezosSyncer{}
	t.syncer = newExternalSyncer(symbol.XTZ)

	return t
}

// GetExtBalance ...
func (ts *TezosSyncer) GetExtBalance() error {
	// If XTZ account exists
	xtsAccount, ok := ts.syncer.Account.EBalances[ts.syncer.assetSymbol]
	if !ok {
		return errors.New("XTZ account does not exists")
	}

	for _, ta := range xtsAccount {
		// TODO: remove empty argument when go-tezos fixes it.
		gt, err := goTezos.NewGoTezos(ts.RPC)
		if err != nil {
			//log.Error().Msgf("Error connecting XTZ RPC: %v", err)
			ts.syncer.addressError[ta.Address] = true
			continue
		}

		// Get latest block number
		latestBlockNumber, err := ts.getLatestBlockNumber(gt)
		if err != nil {
			log.Error().Msgf("Error getting XTZ Latest block from RPC: %v", err)
			ts.syncer.addressError[ta.Address] = true
			continue
		}

		balance, err := gt.Account.GetBalanceAtBlock(ta.Address, latestBlockNumber)
		if err != nil {
			log.Error().Msgf("Error getting XTZ Balance from RPC: %v", err)
			ts.syncer.addressError[ta.Address] = true
			continue
		}
		ts.syncer.ExtBalance[ta.Address] = big.NewInt(int64(balance) * goTezosAccount.MUTEZ)
		ts.syncer.BlockHeight[ta.Address] = latestBlockNumber
		ts.syncer.addressError[ta.Address] = false
	}

	return nil
}

// Update updates accounts in cache as and when external balances
// external chains are updated.
func (ts *TezosSyncer) Update() {
	for _, xtsAccount := range ts.syncer.Account.EBalances[ts.syncer.assetSymbol] {
		if ts.syncer.addressError[xtsAccount.Address] {
			//log.Warn().Msgf("Tezos account info is not available at this moment, skip sync: %s", xtsAccount.Address)
			continue
		}
		ts.syncer.update(xtsAccount.Address)
	}
}

func (ts *TezosSyncer) getLatestBlockNumber(client *goTezos.GoTezos) (*big.Int, error) {
	block, err := client.Block.GetHead()
	if err != nil {
		return nil, err
	}
	return big.NewInt(int64(block.Header.Level)), nil
}
