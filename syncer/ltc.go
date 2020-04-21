package sync

import (
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strconv"

	"github.com/herdius/herdius-core/p2p/log"
	"github.com/herdius/herdius-core/symbol"
)

const mutez = 100000000

// LTCSyncer syncs all XTZ external accounts
type LTCSyncer struct {
	RPC    string
	syncer *ExternalSyncer
}

func newLTCSyncer() *LTCSyncer {
	t := &LTCSyncer{}
	t.syncer = newExternalSyncer(symbol.LTC)

	return t
}

type balanceResponse struct {
	Status string
	Data   struct {
		AvailableBalance string `json:"available_balance"`
	}
}

// GetExtBalance syncs lite coin account.
func (ls *LTCSyncer) GetExtBalance() error {
	lsAccount, ok := ls.syncer.Account.EBalances[ls.syncer.assetSymbol]
	if !ok {
		return errors.New("BTC account does not exists")
	}

	apiKey := os.Getenv("LITECOIN_API_KEY")
	httpClient := newHTTPClient()

	for _, la := range lsAccount {
		ls.syncer.addressError[la.Address] = true
		url := ls.RPC + "/get_balance?address=" + la.Address
		if len(apiKey) > 0 {
			url += "&api_key=" + apiKey
		}
		resp, err := httpClient.Get(url)
		if err != nil {
			//log.Error().Err(err).Msg("Error connecting lite coin network")
			continue
		}

		balanceResp := &balanceResponse{}
		if err := json.NewDecoder(resp.Body).Decode(balanceResp); err != nil {
			log.Error().Err(err).Msg("failed to decode response body")
		}
		if balanceResp.Status == "success" {
			balance, err := strconv.ParseFloat(balanceResp.Data.AvailableBalance, 64)
			if err == nil {
				ls.syncer.addressError[la.Address] = false
				ls.syncer.ExtBalance[la.Address] = big.NewInt(int64(balance * mutez))
				// TODO: implement get block height, nonce?
				ls.syncer.BlockHeight[la.Address] = big.NewInt(0)
			}
		}
		_ = resp.Body.Close()
	}
	return nil
}

// Update updates accounts in cache as and when external balances
// external chains are updated.
func (ls *LTCSyncer) Update() {
	for _, ltcAccount := range ls.syncer.Account.EBalances[ls.syncer.assetSymbol] {
		if ls.syncer.addressError[ltcAccount.Address] {
			//log.Warn().Msgf("LTC account info is not available at this moment, skip sync: %s", ltcAccount.Address)
			continue
		}
		ls.syncer.update(ltcAccount.Address)
	}
}
