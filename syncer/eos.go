package sync

import (
	"encoding/json"
	"errors"
	"math/big"
	"strconv"

	"github.com/herdius/herdius-core/p2p/log"
	"github.com/herdius/herdius-core/symbol"
)

// EOSSyncer syncs all EOS external accounts
type EOSSyncer struct {
	RPC    string
	syncer *ExternalSyncer
}

func newEOSSyncer() *EOSSyncer {
	t := &EOSSyncer{}
	t.syncer = newExternalSyncer(symbol.EOS)

	return t
}

type eosBalance struct {
	Balances []Balance
}

// GetExtBalance syncs eos account.
func (bs *EOSSyncer) GetExtBalance() error {
	bsAccount, ok := bs.syncer.Account.EBalances[bs.syncer.assetSymbol]
	if !ok {
		return errors.New("BTC account does not exists")
	}

	httpClient := newHTTPClient()

	for _, ba := range bsAccount {
		bs.syncer.addressError[ba.Address] = true
		resp, err := httpClient.Get(bs.RPC + "/" + ba.Address)
		if err != nil {
			//log.Error().Err(err).Msg("Error connecting lite coin network")
			continue
		}

		balanceResp := &eosBalance{}
		if err := json.NewDecoder(resp.Body).Decode(balanceResp); err != nil {
			log.Error().Err(err).Msg("failed to decode response body")
		}

		for _, b := range balanceResp.Balances {
			if b.Symbol == "EOS" {
				balance, err := strconv.ParseFloat(b.Free, 64)
				if err == nil {
					bs.syncer.addressError[ba.Address] = false
					bs.syncer.ExtBalance[ba.Address] = big.NewInt(int64(balance * mutez))
					bs.syncer.BlockHeight[ba.Address] = big.NewInt(0)
				}
			}
		}
		_ = resp.Body.Close()
	}
	return nil
}

// Update updates accounts in cache as and when external balances
// external chains are updated.
func (bs *EOSSyncer) Update() {
	for _, eosAccount := range bs.syncer.Account.EBalances[bs.syncer.assetSymbol] {
		if bs.syncer.addressError[eosAccount.Address] {
			continue
		}
		bs.syncer.update(eosAccount.Address)
	}
}
