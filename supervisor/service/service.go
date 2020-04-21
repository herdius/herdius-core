package service

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/herdius/herdius-core/aws"
	"github.com/herdius/herdius-core/blockchain/protobuf"
	cryptokey "github.com/herdius/herdius-core/crypto"
	hehash "github.com/herdius/herdius-core/crypto/herhash"
	"github.com/herdius/herdius-core/crypto/merkle"
	"github.com/herdius/herdius-core/crypto/secp256k1"
	pluginproto "github.com/herdius/herdius-core/hbi/protobuf"
	cmn "github.com/herdius/herdius-core/libs/common"
	cryptokeys "github.com/herdius/herdius-core/p2p/crypto"
	plog "github.com/herdius/herdius-core/p2p/log"
	"github.com/herdius/herdius-core/p2p/network"
	"github.com/herdius/herdius-core/storage/mempool"
	"github.com/herdius/herdius-core/storage/state/statedb"
	"github.com/herdius/herdius-core/supervisor/transaction"
	aSymbol "github.com/herdius/herdius-core/symbol"
	txbyte "github.com/herdius/herdius-core/tx"
)

// SupervisorI is an interface
type SupervisorI interface {
	AddValidator(publicKey []byte, address string) error
	RemoveValidator(address string)
	CreateChildBlock(net *network.Network, txs *transaction.TxList, height int64, previousBlockHash []byte) *protobuf.ChildBlock
	SetWriteMutex()
	SetBackup(bool)
	GetChildBlockMerkleHash() ([]byte, error)
	GetValidatorGroupHash() ([]byte, error)
	GetNextValidatorGroupHash() ([]byte, error)
	CreateBaseBlock(lastBlock *protobuf.BaseBlock) (*protobuf.BaseBlock, error)
	GetMutex() *sync.Mutex
	ProcessTxs(lastBlock *protobuf.BaseBlock, net *network.Network) (*protobuf.BaseBlock, error)
	ShardToValidators(*protobuf.BaseBlock, txbyte.Txs, *network.Network, []byte) (*protobuf.BaseBlock, error)
}

var (
	_ SupervisorI = (*Supervisor)(nil)
)

// Supervisor is concrete implementation of SupervisorI
type Supervisor struct {
	TxBatches           *[]txbyte.Txs // TxGroups will consist of list of the transaction batches
	writerMutex         *sync.Mutex
	ChildBlock          []*protobuf.ChildBlock
	Validator           map[string]*protobuf.Validator
	ValidatorChildblock map[string]*protobuf.BlockID //Validator address pointing to child block hash
	VoteInfoData        map[string][]*protobuf.VoteInfo
	stateRoot           []byte
	env                 string
	waitTime            int
	noOfPeersInGroup    int
	backup              bool
}

// StateRoot returns Supervisor current state root
func (s *Supervisor) StateRoot() []byte {
	return s.stateRoot
}

// SetStateRoot sets Supervisor state root
func (s *Supervisor) SetStateRoot(stateRoot []byte) {
	s.stateRoot = stateRoot
}

// Env returns environment name
func (s *Supervisor) Env() string {
	return s.env
}

// SetEnv sets Supervisor environment
func (s *Supervisor) SetEnv(env string) {
	s.env = env
}

// WaitTime returns wait time value
func (s *Supervisor) WaitTime() int {
	return s.waitTime
}

// SetWaitTime sets Supervisor wait time
func (s *Supervisor) SetWaitTime(waitTime int) {
	s.waitTime = waitTime
}

// SetBackup sets Supervisor backup to S3 process
func (s *Supervisor) SetBackup(backup bool) {
	s.backup = backup
}

// Backup sets Supervisor backup to S3 process
func (s *Supervisor) Backup() bool {
	return s.backup
}

// NoOfPeersInGroup ...
func (s *Supervisor) NoOfPeersInGroup() int {
	return s.noOfPeersInGroup
}

// SetNoOfPeersInGroup ...
func (s *Supervisor) SetNoOfPeersInGroup(n int) {
	s.noOfPeersInGroup = n
}

//GetMutex ...
func (s *Supervisor) GetMutex() *sync.Mutex {
	return s.writerMutex
}

// AddValidator adds a validator to group
func (s *Supervisor) AddValidator(publicKey []byte, address string) error {
	if s.Validator == nil {
		s.Validator = make(map[string]*protobuf.Validator)
	}
	validator := &protobuf.Validator{
		Address:      address,
		PubKey:       publicKey,
		Stakingpower: 100,
	}
	s.writerMutex.Lock()
	s.Validator[address] = validator
	s.writerMutex.Unlock()
	log.Printf("New validator added: <%s>", address)
	return nil
}

// RemoveValidator removes a validator from validators list
func (s *Supervisor) RemoveValidator(address string) {
	s.writerMutex.Lock()
	delete(s.Validator, address)
	s.writerMutex.Unlock()
	log.Printf("Validator removed: <%s>\n", address)
}

// SetWriteMutex ...
func (s *Supervisor) SetWriteMutex() {
	s.writerMutex = new(sync.Mutex)
}

// GetChildBlockMerkleHash creates merkle hash of all the child blocks
func (s *Supervisor) GetChildBlockMerkleHash() ([]byte, error) {
	//cdc.MarshalBinaryBare()
	if s.ChildBlock != nil && len(s.ChildBlock) > 0 {
		cbBzs := make([][]byte, len(s.ChildBlock))

		for i := 0; i < len(s.ChildBlock); i++ {
			cb := s.ChildBlock[i]
			cbBz, err := cdc.MarshalBinaryBare(*cb)
			if err != nil {
				return nil, fmt.Errorf(fmt.Sprintf("Child block Marshaling failed: %v.", err))
			}
			cbBzs[i] = cbBz
		}

		return merkle.SimpleHashFromByteSlices(cbBzs), nil
	}
	return nil, fmt.Errorf("no Child block available: %v", s.ChildBlock)
}

// GetValidatorGroupHash creates merkle hash of all the validators
func (s *Supervisor) GetValidatorGroupHash() ([]byte, error) {
	numValidators := len(s.Validator)
	if numValidators == 0 {
		return nil, fmt.Errorf(fmt.Sprintf("No Child block available: %v.", s.ChildBlock))
	}
	vlBzs := make([][]byte, numValidators)

	for _, validator := range s.Validator {
		vl := validator
		vlBz, err := cdc.MarshalBinaryBare(*vl)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Validator Marshaling failed: %v.", err))
		}
		vlBzs = append(vlBzs, vlBz)
	}

	return merkle.SimpleHashFromByteSlices(vlBzs), nil
}

// GetNextValidatorGroupHash ([]byte, error) creates merkle hash of all the next validators
// TODO: refine logic, currently, it's the same as s.GetValidatorGroupHash.
func (s *Supervisor) GetNextValidatorGroupHash() ([]byte, error) {
	return s.GetValidatorGroupHash()
}

// CreateBaseBlock creates the base block with all the child blocks
func (s *Supervisor) CreateBaseBlock(lastBlock *protobuf.BaseBlock) (*protobuf.BaseBlock, error) {
	// Create the merkle hash of all the child blocks
	cbMerkleHash, err := s.GetChildBlockMerkleHash()
	if err != nil {
		plog.Error().Msgf("Failed to create Merkle Hash of Validators: %v", err)
	}

	// Create the merkle hash of all the validators
	vgHash, err := s.GetValidatorGroupHash()
	if err != nil {
		plog.Error().Msgf("Failed to create Merkle Hash of Validators: %v", err)
	}

	// Create the merkle hash of all the next validators
	nvgHash, err := s.GetNextValidatorGroupHash()
	if err != nil {
		plog.Error().Msgf("Failed to create Merkle Hash of Next Validators: %v", err)
	}

	height := lastBlock.GetHeader().GetHeight()

	// create array of vote commits
	votecommits := make([]protobuf.VoteCommit, 0)
	for _, v := range s.ChildBlock {
		var cbh cmn.HexBytes = v.GetHeader().GetBlockID().GetBlockHash()
		groupVoteInfo := s.VoteInfoData[cbh.String()]

		voteCommit := protobuf.VoteCommit{
			BlockID: v.GetHeader().GetBlockID(),
			Vote:    groupVoteInfo,
		}
		votecommits = append(votecommits, voteCommit)
	}

	vcbz, err := cdc.MarshalJSON(votecommits)
	if err != nil {
		plog.Error().Msgf("Vote commits marshaling failed.: %v", err)
	}

	ts := time.Now().UTC()
	baseHeader := &protobuf.BaseHeader{
		Block_ID:               &protobuf.BlockID{},
		LastBlockID:            lastBlock.GetHeader().GetBlock_ID(),
		Height:                 height + 1,
		ValidatorGroupHash:     vgHash,
		NextValidatorGroupHash: nvgHash,
		ChildBlockHash:         cbMerkleHash,
		LastVoteHash:           vcbz,
		StateRoot:              s.stateRoot,
		Time: &protobuf.Timestamp{
			Seconds: ts.Unix(),
			Nanos:   ts.UnixNano(),
		},
	}

	blockHashBz, err := cdc.MarshalJSON(baseHeader)
	if err != nil {
		plog.Error().Msgf("Base Header marshaling failed.: %v", err)
	}
	blockHash := hehash.Sum(blockHashBz)

	baseHeader.GetBlock_ID().BlockHash = blockHash

	childBlocksBz, err := cdc.MarshalJSON(s.ChildBlock)
	if err != nil {
		plog.Error().Msgf("Child blocks marshaling failed.: %v", err)
	}

	// Vote commits marshalling
	vcBz, err := cdc.MarshalJSON(votecommits)
	if err != nil {
		plog.Error().Msgf("Vote Commits marshaling failed.: %v", err)
	}

	// Validators marshaling
	valsBz, err := cdc.MarshalJSON(s.Validator)
	if err != nil {
		plog.Error().Msgf("Validators marshaling failed.: %v", err)
	}
	s.writerMutex.Lock()
	baseBlock := &protobuf.BaseBlock{
		Header:        baseHeader,
		ChildBlock:    childBlocksBz,
		VoteCommits:   vcBz,
		Validator:     valsBz,
		NextValidator: valsBz,
	}
	s.writerMutex.Unlock()
	return baseBlock, nil
}

// CreateChildBlock creates an initial child block
func (s *Supervisor) CreateChildBlock(net *network.Network, txs *transaction.TxList, height int64, previousBlockHash []byte) *protobuf.ChildBlock {
	txList := *txs
	if len(txList.Transactions) == 0 {
		return nil
	}

	numTxs := len(txList.Transactions)

	txbzs := make([][]byte, 0)
	txservice := txbyte.GetTxsService()

	for _, tx := range txList.Transactions {

		txbz, err := cdc.MarshalJSON(*tx)
		//plog.Fatalf("Marshalling failed: %v.", err)
		if err != nil {
			return nil
		}
		txbzs = append(txbzs, txbz)
	}
	txservice.SetTxs(txbzs)

	// Get Merkle Root Hash of all transactions
	rootHash := txservice.MerkleHash()

	//Supervisor details
	var keys *cryptokeys.KeyPair
	pubKey := make([]byte, 0)
	var address string
	if net != nil {
		keys = net.GetKeys()
		pubKey = keys.PubKey.Bytes()
		address = keys.PubKey.GetAddress()
	}

	// TODO: Id value calculation needs to implemented.
	id := &protobuf.ID{
		PublicKey: pubKey,
		Address:   address,
		Id:        []byte{0},
	}

	lastBlockID := &protobuf.BlockID{
		BlockHash: previousBlockHash,
	}
	// Create the child block
	header := &protobuf.Header{
		SupervisorID: id,
		NumTxs:       int64(numTxs),
		TotalTxs:     int64(numTxs),
		RootHash:     rootHash,
		Height:       height,
		LastBlockID:  lastBlockID,
	}
	hbz, _ := cdc.MarshalJSON(header)

	// Create the SHA256 value of the header
	// SHA256 Block Hash value is calculated using below header details:
	// Supervisor ID, # of txs, total txs and root hash
	// TODO: Need to make it better
	blockhash := hehash.Sum(hbz)

	blockID := &protobuf.BlockID{
		BlockHash: blockhash,
	}

	header.BlockID = blockID
	txsData := &protobuf.TxsData{
		Tx: txbzs,
	}
	s.writerMutex.Lock()
	cb := &protobuf.ChildBlock{
		Header:  header,
		TxsData: txsData,
	}
	s.writerMutex.Unlock()
	return cb
}

// ProcessTxs will process transactions.
// It will check whether to send the transactions to Validators
// or to be included in Singular base block
func (s *Supervisor) ProcessTxs(lastBlock *protobuf.BaseBlock, net *network.Network) (*protobuf.BaseBlock, error) {
	mp := mempool.GetMemPool()
	select {
	case <-time.After(time.Duration(s.waitTime) * time.Second):
		txs := mp.GetTxs()
		if len(s.Validator) == 0 || len(*txs) == 0 {
			log.Printf("Block creation wait time (%d) elapsed, creating singular base block but with %v transactions", s.waitTime, len(*txs))
			baseBlock, err := s.createSingularBlock(lastBlock, net, *txs, mp, s.stateRoot)
			if err != nil {
				return nil, fmt.Errorf("failed to create singular base block: %v", err)
			}
			mp.RemoveTxs(len(*txs))
			if !s.Backup() {
				log.Println("Backup value false, not backing up block or state")
				return baseBlock, nil
			}
			backuper := aws.NewBackuper(s.env)
			succ, err := backuper.TryBackupBaseBlock(lastBlock, baseBlock)
			if err != nil {
				log.Println("nonfatal: failed to backup to S3:", err)
			} else if !succ {
				log.Println("S3 backup criteria not met; proceeding to backup all unbacked base blocks")
				err := backuper.BackupNeededBaseBlocks(baseBlock)
				if err != nil {
					log.Println("nonfatal: failed to backup both single new and all unbacked base blocks:", err)
				}
				log.Print("Successfully re-evaluated chain and backed up to S3")
			}

			return baseBlock, nil
		}
		baseBlock, err := s.ShardToValidators(lastBlock, *txs, net, s.stateRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to shard Txs to child blocks: %v", err)
		}
		mp.RemoveTxs(len(*txs))
		return baseBlock, nil
	}
}

func (s *Supervisor) createSingularBlock(lastBlock *protobuf.BaseBlock, net *network.Network, txs txbyte.Txs, mp *mempool.MemPool, stateRoot []byte) (*protobuf.BaseBlock, error) {
	stateTrie, err := statedb.NewTrie(common.BytesToHash(stateRoot))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the state trie: %v", err)
	}
	if accountStorage != nil {
		stateTrie = updateStateWithNewExternalBalance(stateTrie)
	}
	if _, err := s.updateStateForTxs(&txs, stateTrie); err != nil {
		return nil, fmt.Errorf("failed to update state for txs: %v", err)
	}

	// Get Merkle Root Hash of all transactions
	mrh := txs.MerkleHash()

	// Create Singular Block Header
	ts := time.Now().UTC()
	baseHeader := &protobuf.BaseHeader{
		Block_ID:    &protobuf.BlockID{},
		LastBlockID: lastBlock.GetHeader().GetBlock_ID(),
		Height:      lastBlock.Header.Height + 1,
		StateRoot:   s.stateRoot,
		Time: &protobuf.Timestamp{
			Seconds: ts.Unix(),
			Nanos:   ts.UnixNano(),
		},
		RootHash: mrh,
		TotalTxs: uint64(len(txs)),
	}
	blockHashBz, err := cdc.MarshalJSON(baseHeader)
	if err != nil {
		plog.Error().Msgf("Base Header marshaling failed.: %v", err)
	}

	blockHash := hehash.Sum(blockHashBz)
	baseHeader.GetBlock_ID().BlockHash = blockHash
	// Add Header to Block

	s.writerMutex.Lock()
	baseBlock := &protobuf.BaseBlock{
		Header:  baseHeader,
		TxsData: &protobuf.TxsData{Tx: txs},
	}
	s.writerMutex.Unlock()

	return baseBlock, nil
}

func updateStateWithNewExternalBalance(stateTrie statedb.Trie) statedb.Trie {
	updateAccs := accountStorage.GetAll()
	log.Println("Total Accounts to update", len(updateAccs))
	for address, item := range updateAccs {
		accountInAccountCache := item
		account := item.Account
		for assetSymbol := range account.EBalances {
			for _, eb := range account.EBalances[assetSymbol] {
				storageKey := assetSymbol + "-" + eb.Address
				IsFirstEntry := item.IsFirstEntry[storageKey]
				IsNewAmountUpdate := item.IsNewAmountUpdate[storageKey]
				if IsNewAmountUpdate && !IsFirstEntry {
					//log.Printf("Account from cache to be persisted to state: %v", account)
					sactbz, err := cdc.MarshalJSON(account)
					if err != nil {
						plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
						continue
					}
					stateTrie.TryUpdate([]byte(address), sactbz)
					accountInAccountCache.IsNewAmountUpdate[storageKey] = false
					accountStorage.Set(address, accountInAccountCache)
				}
				if IsFirstEntry {
					//log.Println("Account from cache to be persisted to state first time: ", account)
					sactbz, err := cdc.MarshalJSON(account)
					if err != nil {
						plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
						continue
					}
					stateTrie.TryUpdate([]byte(address), sactbz)
					accountInAccountCache.IsFirstEntry[storageKey] = false
					accountStorage.Set(address, accountInAccountCache)
				}
			}
		}

		// IF ERC20Address is presend update accoun balance
		if len(account.Erc20Address) > 0 {
			IsFirstEntry := item.IsFirstHEREntry
			IsNewAmountUpdate := item.IsNewHERAmountUpdate
			if IsNewAmountUpdate && !IsFirstEntry {
				//log.Printf("Account from cache to be persisted to state: %v", account)
				sactbz, err := cdc.MarshalJSON(account)
				if err != nil {
					plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
					continue
				}
				stateTrie.TryUpdate([]byte(address), sactbz)
				accountInAccountCache.IsNewHERAmountUpdate = false
				accountStorage.Set(address, accountInAccountCache)
			}
			if IsFirstEntry {
				//log.Println("Account from cache to be persisted to state first time: ", account)
				sactbz, err := cdc.MarshalJSON(account)
				if err != nil {
					plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
					continue
				}
				stateTrie.TryUpdate([]byte(address), sactbz)
				accountInAccountCache.IsFirstHEREntry = false
				accountStorage.Set(address, accountInAccountCache)
			}

		}

	}
	return stateTrie
}
func isExternalAssetAddressExist(account *statedb.Account, assetSymbol, assetAddress string) bool {
	if account == nil || account.EBalances == nil {
		return false
	}
	if len(account.EBalances[assetSymbol][assetAddress].Address) > 0 {
		return true
	}
	return false
}

func updateAccountLockedBalance(senderAccount *statedb.Account, tx *pluginproto.Tx) *statedb.Account {
	if senderAccount.LockedBalance == nil {
		senderAccount.LockedBalance = make(map[string]map[string]uint64)
	}
	asset := strings.ToUpper(tx.Asset.Symbol)
	if senderAccount.LockedBalance[asset] == nil {
		senderAccount.LockedBalance[asset] = make(map[string]uint64)
	}

	if tx.SenderAddress == senderAccount.Address {
		senderAccount.LockedBalance[asset][tx.Asset.ExternalSenderAddress] += tx.Asset.LockedAmount
	}
	withdraw(senderAccount, tx.Asset.Symbol, tx.Asset.ExternalSenderAddress, tx.Asset.LockedAmount)
	senderAccount.Nonce = tx.Asset.Nonce
	if strings.EqualFold(aSymbol.BTC, tx.Asset.Symbol) {
		if _, ok := senderAccount.EBalances[aSymbol.HBTC]; !ok {
			eBalance := statedb.EBalance{}
			eBalance.Address = senderAccount.FirstExternalAddress[aSymbol.ETH]
			eBalance.Balance = 0
			eBalance.LastBlockHeight = 0
			eBalance.Nonce = 1
			eBalances := senderAccount.EBalances
			eBalances[aSymbol.HBTC] = make(map[string]statedb.EBalance)
			eBalances[aSymbol.HBTC][senderAccount.FirstExternalAddress[aSymbol.ETH]] = eBalance
			senderAccount.EBalances = eBalances
		}
	}

	if strings.EqualFold(aSymbol.BNB, tx.Asset.Symbol) {
		if _, ok := senderAccount.EBalances[aSymbol.HBNB]; !ok {
			eBalance := statedb.EBalance{}
			eBalance.Address = senderAccount.FirstExternalAddress[aSymbol.ETH]
			eBalance.Balance = 0
			eBalance.LastBlockHeight = 0
			eBalance.Nonce = 1
			eBalances := senderAccount.EBalances
			eBalances[aSymbol.HBNB] = make(map[string]statedb.EBalance)
			eBalances[aSymbol.HBNB][senderAccount.FirstExternalAddress[aSymbol.ETH]] = eBalance
			senderAccount.EBalances = eBalances
		}
	}

	if strings.EqualFold(aSymbol.LTC, tx.Asset.Symbol) {
		if _, ok := senderAccount.EBalances[aSymbol.HLTC]; !ok {
			eBalance := statedb.EBalance{}
			eBalance.Address = senderAccount.FirstExternalAddress[aSymbol.ETH]
			eBalance.Balance = 0
			eBalance.LastBlockHeight = 0
			eBalance.Nonce = 1
			eBalances := senderAccount.EBalances
			eBalances[aSymbol.HLTC] = make(map[string]statedb.EBalance)
			eBalances[aSymbol.HLTC][senderAccount.FirstExternalAddress[aSymbol.ETH]] = eBalance
			senderAccount.EBalances = eBalances
		}
	}

	if strings.EqualFold(aSymbol.XTZ, tx.Asset.Symbol) {
		if _, ok := senderAccount.EBalances[aSymbol.HXTZ]; !ok {
			eBalance := statedb.EBalance{}
			eBalance.Address = senderAccount.FirstExternalAddress[aSymbol.ETH]
			eBalance.Balance = 0
			eBalance.LastBlockHeight = 0
			eBalance.Nonce = 1
			eBalances := senderAccount.EBalances
			eBalances[aSymbol.HXTZ] = make(map[string]statedb.EBalance)
			eBalances[aSymbol.HXTZ][senderAccount.FirstExternalAddress[aSymbol.ETH]] = eBalance
			senderAccount.EBalances = eBalances
		}
	}

	log.Printf("Locked Account: %v+\n", *senderAccount)
	return senderAccount
}

func updateRedeemAccountLockedBalance(senderAccount *statedb.Account, tx *pluginproto.Tx) *statedb.Account {
	if senderAccount.LockedBalance == nil {
		return senderAccount
	}
	asset := strings.ToUpper(tx.Asset.Symbol)
	firstExternalAddress := ""
	switch strings.ToUpper(tx.Asset.Symbol) {
	case aSymbol.HBTC:
		asset = aSymbol.BTC
		firstExternalAddress = senderAccount.FirstExternalAddress[aSymbol.ETH]
	case aSymbol.HBNB:
		asset = aSymbol.BNB
		firstExternalAddress = senderAccount.FirstExternalAddress[aSymbol.ETH]
	case aSymbol.HLTC:
		asset = aSymbol.LTC
		firstExternalAddress = senderAccount.FirstExternalAddress[aSymbol.ETH]
	case aSymbol.HXTZ:
		asset = aSymbol.XTZ
		firstExternalAddress = senderAccount.FirstExternalAddress[aSymbol.ETH]
	}

	if firstExternalAddress == "" {
		return senderAccount
	}

	if senderAccount.LockedBalance[asset] == nil {
		if strings.EqualFold(tx.Asset.Symbol, aSymbol.HBTC) {
			// New HBTC Balance update
			firstExternalAddress := senderAccount.FirstExternalAddress[aSymbol.ETH]
			var newHBTCExternalBal uint64
			if senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance > tx.Asset.RedeemedAmount {
				newHBTCExternalBal = senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance - tx.Asset.RedeemedAmount
			} else {
				newHBTCExternalBal = 0
			}
			newHBTCEBal := statedb.EBalance{
				Address:         firstExternalAddress,
				Balance:         newHBTCExternalBal,
				LastBlockHeight: senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].LastBlockHeight,
				Nonce:           senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].Nonce,
			}
			senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress] = newHBTCEBal
		}

		if strings.EqualFold(tx.Asset.Symbol, aSymbol.HBNB) {
			// New HBNB Balance update
			firstExternalAddress := senderAccount.FirstExternalAddress[aSymbol.ETH]
			var newHBNBExternalBal uint64
			if senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance > tx.Asset.RedeemedAmount {
				newHBNBExternalBal = senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance - tx.Asset.RedeemedAmount
			} else {
				newHBNBExternalBal = 0
			}
			newHBNBEBal := statedb.EBalance{
				Address:         firstExternalAddress,
				Balance:         newHBNBExternalBal,
				LastBlockHeight: senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].LastBlockHeight,
				Nonce:           senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].Nonce,
			}
			senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress] = newHBNBEBal
		}

		if strings.EqualFold(tx.Asset.Symbol, aSymbol.HXTZ) {
			// New HXTZ Balance update
			firstExternalAddress := senderAccount.FirstExternalAddress[aSymbol.ETH]
			var newHXTZExternalBal uint64
			if senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance > tx.Asset.RedeemedAmount {
				newHXTZExternalBal = senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance - tx.Asset.RedeemedAmount
			} else {
				newHXTZExternalBal = 0
			}
			newHXTZEBal := statedb.EBalance{
				Address:         firstExternalAddress,
				Balance:         newHXTZExternalBal,
				LastBlockHeight: senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].LastBlockHeight,
				Nonce:           senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].Nonce,
			}
			senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress] = newHXTZEBal
		}

		if strings.EqualFold(tx.Asset.Symbol, aSymbol.HLTC) {
			// New HLTC Balance update
			firstExternalAddress := senderAccount.FirstExternalAddress[aSymbol.ETH]
			var newExternalBal uint64
			if senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance > tx.Asset.RedeemedAmount {
				newExternalBal = senderAccount.EBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress].Balance - tx.Asset.RedeemedAmount
			} else {
				newExternalBal = 0
			}
			newHLTCEBal := statedb.EBalance{
				Address:         firstExternalAddress,
				Balance:         newExternalBal,
				LastBlockHeight: senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].LastBlockHeight,
				Nonce:           senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress].Nonce,
			}
			senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress] = newHLTCEBal
		}
		return senderAccount

	} else if tx.SenderAddress == senderAccount.Address &&
		tx.Asset.RedeemedAmount <= senderAccount.LockedBalance[asset][tx.Asset.ExternalSenderAddress] {

		// Update locked balance
		if senderAccount.LockedBalance[asset][tx.Asset.ExternalSenderAddress] > tx.Asset.RedeemedAmount {
			senderAccount.LockedBalance[asset][tx.Asset.ExternalSenderAddress] -= tx.Asset.RedeemedAmount
		} else {
			senderAccount.LockedBalance[asset][tx.Asset.ExternalSenderAddress] = 0
		}

		// Update symbol balance
		eBalance := senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress]
		eBalance.Balance -= tx.Asset.RedeemedAmount
		senderAccount.EBalances[tx.Asset.Symbol][firstExternalAddress] = eBalance
	}
	senderAccount.Nonce = tx.Asset.Nonce
	deposit(senderAccount, asset, tx.Asset.ExternalSenderAddress, tx.Asset.RedeemedAmount)
	log.Printf("Redeemed Account: %v+\n", *senderAccount)
	return senderAccount
}

// registerAccount will create register a new account to herdius blockchain
// along with all the supported addresses
func registerAccount(senderAccount *statedb.Account, tx *pluginproto.Tx) *statedb.Account {
	log.Printf("New account register: %+v \n", tx)
	senderAccount.Address = tx.SenderAddress
	senderAccount.Balance = 0
	senderAccount.Nonce = 0
	senderAccount.PublicKey = tx.SenderPubkey
	senderAccount.Erc20Address = tx.Asset.ExternalSenderAddress
	senderAccount.FirstExternalAddress = make(map[string]string)
	senderAccount.EBalances = make(map[string]map[string]statedb.EBalance)

	for symbol, address := range tx.ExternalAddress {
		log.Println(symbol + " address ( " + address + " ) added.")
		eBalance := statedb.EBalance{}
		eBalance.Address = address
		eBalance.Balance = 0
		eBalance.LastBlockHeight = 0
		eBalance.Nonce = 0
		eBalances := senderAccount.EBalances
		if len(eBalances[symbol]) == 0 {
			eBalances[symbol] = make(map[string]statedb.EBalance)
		}
		eBalances[symbol][address] = eBalance
		senderAccount.EBalances = eBalances
		senderAccount.FirstExternalAddress[symbol] = address
	}
	return senderAccount
}

func updateAccount(senderAccount *statedb.Account, tx *pluginproto.Tx) *statedb.Account {
	if strings.EqualFold(strings.ToUpper(tx.Asset.Symbol), aSymbol.HER) &&
		len(senderAccount.Address) == 0 {
		senderAccount.Address = tx.SenderAddress
		senderAccount.Balance = 0
		senderAccount.Nonce = 0
		senderAccount.PublicKey = tx.SenderPubkey
		log.Println("Account register", tx)
		senderAccount.Erc20Address = tx.Asset.ExternalSenderAddress
		senderAccount.FirstExternalAddress = make(map[string]string)
	} else if strings.EqualFold(strings.ToUpper(tx.Asset.Symbol), aSymbol.HER) &&
		tx.SenderAddress == senderAccount.Address {
		senderAccount.Balance += tx.Asset.Value
		senderAccount.Nonce = tx.Asset.Nonce
	} else if !strings.EqualFold(strings.ToUpper(tx.Asset.Symbol), aSymbol.HER) &&
		tx.SenderAddress == senderAccount.Address {

		// Update account's Nonce
		senderAccount.Nonce = tx.Asset.Nonce

		// Register External Asset Addresses if not exist
		if assetEBalance, ok := senderAccount.EBalances[tx.Asset.Symbol]; ok {
			if _, ok := assetEBalance[tx.Asset.ExternalSenderAddress]; !ok {
				eBalance := statedb.EBalance{}
				eBalance.Address = tx.Asset.ExternalSenderAddress
				eBalance.Balance = 0
				eBalance.LastBlockHeight = 0
				eBalance.Nonce = 0
				eBalances := senderAccount.EBalances
				eBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress] = eBalance
				senderAccount.EBalances = eBalances
			}
		} else {
			eBalance := statedb.EBalance{}
			eBalance.Address = tx.Asset.ExternalSenderAddress
			eBalance.Balance = 0
			eBalance.LastBlockHeight = 0
			eBalance.Nonce = 0
			eBalances := senderAccount.EBalances
			if len(eBalances) == 0 {
				eBalances = make(map[string]map[string]statedb.EBalance)
			}
			if len(eBalances[tx.Asset.Symbol]) == 0 {
				eBalances[tx.Asset.Symbol] = make(map[string]statedb.EBalance)
			}
			eBalances[tx.Asset.Symbol][tx.Asset.ExternalSenderAddress] = eBalance
			senderAccount.EBalances = eBalances
			if senderAccount.FirstExternalAddress == nil {
				senderAccount.FirstExternalAddress = make(map[string]string)
			}
			senderAccount.FirstExternalAddress[tx.Asset.Symbol] = tx.Asset.ExternalSenderAddress
		}
	}
	return senderAccount
}

// Debit Sender's Account
func withdraw(senderAccount *statedb.Account, assetSymbol, assetExtAddress string, txValue uint64) {
	if strings.EqualFold(assetSymbol, aSymbol.HER) {
		balance := senderAccount.Balance
		if balance >= txValue {
			senderAccount.Balance -= txValue
		}
	} else {
		// Get balance of the required external asset
		eBalance := senderAccount.EBalances[strings.ToUpper(assetSymbol)][assetExtAddress]
		if eBalance.Balance >= txValue {
			eBalance.Balance -= txValue
			senderAccount.EBalances[strings.ToUpper(assetSymbol)][assetExtAddress] = eBalance
		}
	}
}

// Credit Receiver's Account
func deposit(receiverAccount *statedb.Account, assetSymbol, assetExtAddress string, txValue uint64) {
	if strings.EqualFold(assetSymbol, aSymbol.HER) {
		receiverAccount.Balance += txValue
	} else {
		// Get balance of the required external asset
		eBalance := receiverAccount.EBalances[strings.ToUpper(assetSymbol)][assetExtAddress]
		eBalance.Balance += txValue
		receiverAccount.EBalances[strings.ToUpper(assetSymbol)][assetExtAddress] = eBalance
	}
}

func (s *Supervisor) validatorAddresses() []string {
	addresses := make([]string, len(s.Validator))
	for address := range s.Validator {
		addresses = append(addresses, address)
	}
	return addresses
}

func (s *Supervisor) validatorGroups(numGroup int) [][]string {
	s.writerMutex.Lock()
	defer s.writerMutex.Unlock()

	numValds := len(s.Validator)
	validators := make([]string, 0)
	for address := range s.Validator {
		validators = append(validators, address)
	}

	var groups [][]string
	if numGroup <= 0 {
		numGroup = 1
	}
	groupSize := (numValds + numGroup - 1) / numGroup

	for start := 0; start < numValds; start += groupSize {
		end := start + groupSize
		if end > numValds {
			end = numValds
		}
		groups = append(groups, validators[start:end])
	}

	return groups
}

func (s *Supervisor) txsGroups(txList *transaction.TxList, numGroup int) [][]*transaction.Tx {
	txs := txList.Transactions

	var groups [][]*transaction.Tx
	groupSize := (len(txs) + numGroup - 1) / numGroup

	for start := 0; start < len(txs); start += groupSize {
		end := start + groupSize
		if end > len(txs) {
			end = len(txs)
		}
		groups = append(groups, txs[start:end])
	}

	return groups
}

// ShardToValidators distributes a series of childblocks to a series of validators
func (s *Supervisor) ShardToValidators(lastBlock *protobuf.BaseBlock, txs txbyte.Txs, net *network.Network, stateRoot []byte) (*protobuf.BaseBlock, error) {
	numValds := len(s.Validator)
	if numValds == 0 {
		return nil, fmt.Errorf("not enough validators in pool to shard, # validators: %v", numValds)
	}
	numTxs := len(txs)
	// TODO: Make number of groups configurable.
	numGroup := 5
	vGroups := s.validatorGroups(numGroup)
	fmt.Printf("Number of txs (%v), groups (%v), validators (%v)\n", numTxs, vGroups, numValds)

	if len(stateRoot) == 0 {
		return nil, fmt.Errorf("cannot process an empty stateRoot for the trie")
	}
	stateTrie, err := statedb.NewTrie(common.BytesToHash(stateRoot))
	if err != nil {
		return nil, fmt.Errorf("error attempting to retrieve state db trie from stateRoot: %v", err)
	}
	if accountStorage != nil {
		stateTrie = updateStateWithNewExternalBalance(stateTrie)
	}
	txList, err := s.updateStateForTxs(&txs, stateTrie)
	if err != nil {
		return nil, fmt.Errorf("failed to update state for txs: %v", err)
	}

	txsGroups := s.txsGroups(txList, numGroup)
	previousBlockHash := make([]byte, 0)
	var voteCount = 0
	for i := range txsGroups {
		cb := s.CreateChildBlock(net, &transaction.TxList{Transactions: txsGroups[i]}, int64(len(txsGroups[i])), previousBlockHash)
		previousBlockHash = cb.GetHeader().GetBlockID().BlockHash
		if i > 0 {
			cb.GetHeader().GetLastBlockID().BlockHash = previousBlockHash
		}
		cbmsg := &protobuf.ChildBlockMessage{ChildBlock: cb}
		log.Println("Broadcasting child block to Validator Group:", vGroups[i])
		for _, address := range vGroups[i] {
			validator, err := net.Client(address)
			if err != nil {
				return nil, fmt.Errorf("failed to create validator peer client: %v", err)
			}
			if validator.Address == "" {
				log.Println("Empty validator node:", validator)
				return nil, fmt.Errorf("Empty validator node:: %v", validator)
			}
			ctx := network.WithSignMessage(context.Background(), true)
			response, err := validator.Request(ctx, cbmsg)
			if err != nil {
				return nil, fmt.Errorf(fmt.Sprintf("Failed to find block due to: %v", err))
			}
			switch msg := response.(type) {
			case *protobuf.ChildBlockMessage:
				mcb := msg
				vote := mcb.GetVote()
				if vote != nil {
					// Increment the vote count of validator group
					voteCount++

					var cbhash cmn.HexBytes
					cbhash = mcb.GetChildBlock().GetHeader().GetBlockID().GetBlockHash()
					voteinfo := s.VoteInfoData[cbhash.String()]
					voteinfo = append(voteinfo, vote)
					s.VoteInfoData[cbhash.String()] = voteinfo

					sign := vote.GetSignature()
					var pubKey cryptokey.PubKey

					cdc.UnmarshalBinaryBare(vote.GetValidator().GetPubKey(), &pubKey)

					isVerified := pubKey.VerifyBytes(vote.GetValidator().GetPubKey(), sign)

					isChildBlockSigned := mcb.GetVote().GetSignedCurrentBlock()

					// Check whether Childblock is verified and signed by the validator
					if isChildBlockSigned && isVerified {
						mx := s.GetMutex()
						mx.Lock()
						s.ValidatorChildblock[address] = mcb.GetChildBlock().GetHeader().GetBlockID()
						mx.Unlock()
						log.Printf("<%s> Validator verified and signed the child block: %v", address, isVerified)
						s.ChildBlock = append(s.ChildBlock, mcb.GetChildBlock())
					}
				}
			}
		}
	}

	if voteCount == len(s.Validator) {
		baseBlock, err := s.CreateBaseBlock(lastBlock)
		if err != nil {
			return nil, err
		}
		return baseBlock, nil
	}

	return nil, nil
}

func (s *Supervisor) updateStateForTxs(txs *txbyte.Txs, stateTrie statedb.Trie) (*transaction.TxList, error) {
	txStr := transaction.Tx{}
	txlist := &transaction.TxList{}
	tx := pluginproto.Tx{}
	for i, txbz := range *txs {
		err := cdc.UnmarshalJSON(txbz, &txStr)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal tx: %v", err)
		}

		err = cdc.UnmarshalJSON(txbz, &tx)
		if err != nil {
			log.Printf("Failed to Unmarshal tx: %v", err)
			continue
		}

		// Get the public key of the sender
		senderAddress := tx.GetSenderAddress()
		pubKeyS, err := b64.StdEncoding.DecodeString(tx.GetSenderPubkey())
		if err != nil {
			log.Printf("Failed to decode sender public key: %v", err)
			plog.Error().Msgf("Failed to decode sender public key: %v", err)
			tx.Status = "failed"
			txbz, err = cdc.MarshalJSON(&tx)
			(*txs)[i] = txbz
			txStr.Status = tx.Status
			txlist.Transactions = append(txlist.Transactions, &txStr)
			if err != nil {
				log.Printf("Failed to encode failed tx: %v", err)
				plog.Error().Msgf("Failed to encode failed tx: %v", err)
			}
			continue
		}

		var pubKey secp256k1.PubKeySecp256k1
		copy(pubKey[:], pubKeyS)

		signatureVerified, err := txbyte.VerifySignature(&tx, pubKey)
		if err != nil {
			plog.Error().Msgf("failed to verify tx signature: %v", err)
			continue
		}
		if !signatureVerified {
			plog.Error().Msg("Signature Verification Failed")
			tx.Status = "failed"
			txbz, err = cdc.MarshalJSON(&tx)
			(*txs)[i] = txbz
			txStr.Status = tx.Status
			txlist.Transactions = append(txlist.Transactions, &txStr)
			if err != nil {
				plog.Error().Msgf("Failed to encode failed tx: %v", err)
				log.Printf("Failed to encode failed tx: %v", err)
			}
			continue
		}
		var senderAccount statedb.Account
		senderAddressBytes := []byte(senderAddress)

		// Get account details from state trie
		senderActbz, err := stateTrie.TryGet(senderAddressBytes)
		if err != nil {
			plog.Error().Msgf("Failed to retrieve account detail: %v", err)
			log.Printf("Failed to retrieve account detail: %v", err)
			continue
		}

		if len(senderActbz) > 0 {
			err = cdc.UnmarshalJSON(senderActbz, &senderAccount)
			if err != nil {
				log.Printf("Failed to Unmarshal account: %v", err)
				plog.Error().Msgf("Failed to Unmarshal account: %v", err)
				continue
			}
		}

		// Check if tx is of type account update
		if strings.EqualFold(tx.Asset.Network, "Fabric") ||
			strings.EqualFold(tx.Asset.Network, "Corda") {
			senderAccount.Nonce = tx.Asset.Nonce

			sactbz, err := cdc.MarshalJSON(senderAccount)
			if err != nil {
				log.Printf("Failed to Marshal sender's account: %v", err)
				plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
				continue
			}
			addressBytes := []byte(pubKey.GetAddress())
			err = stateTrie.TryUpdate(addressBytes, sactbz)
			if err != nil {
				log.Printf("Failed to store account in state db: %v", err)
				plog.Error().Msgf("Failed to store account in state db: %v", err)
				tx.Status = "failed"
				txbz, err = cdc.MarshalJSON(&tx)
				(*txs)[i] = txbz
				if err != nil {
					log.Printf("Failed to encode failed tx: %v", err)
					plog.Error().Msgf("Failed to encode failed tx: %v", err)
					continue
				}
			}
			tx.Status = "success"
			txbz, err = cdc.MarshalJSON(&tx)
			txStr.Status = tx.Status
			txlist.Transactions = append(txlist.Transactions, &txStr)
			(*txs)[i] = txbz
			if err != nil {
				log.Printf("Failed to encode failed tx: %v", err)
				plog.Error().Msgf("Failed to encode failed tx: %v", err)
			}

			continue
		} else if strings.EqualFold(tx.Type, "External") ||
			strings.EqualFold(tx.Type, "Lend") ||
			strings.EqualFold(tx.Type, "Borrow") {
			symbol := tx.Asset.Symbol
			if symbol != aSymbol.BTC && symbol != aSymbol.ETH && symbol != aSymbol.HBTC && symbol != aSymbol.XTZ {
				log.Printf("Unsupported external asset symbol: %v", symbol)
				plog.Error().Msgf("Unsupported external asset symbol: %v", symbol)
				continue
			}

			// By default each of the new accounts will have HER token (with balance 0)
			// added to the map object balances
			balance := senderAccount.EBalances[symbol][tx.Asset.ExternalSenderAddress]
			if balance == (statedb.EBalance{}) {
				plog.Error().Msgf("Sender has no assets for the given symbol: %v", symbol)
				log.Printf("Sender has no assets for the given symbol: %v", symbol)
				continue
			}
			if balance.Balance < tx.Asset.Value {
				plog.Error().Msgf("Sender does not have enough assets in account (%d) to send transaction amount (%d)", balance.Balance, tx.Asset.Value)
				log.Printf("Sender does not have enough assets in account (%d) to send transaction amount (%d)", balance.Balance, tx.Asset.Value)
				continue
			}

			if strings.EqualFold(tx.Type, "External") ||
				(strings.EqualFold(tx.Type, "Lend") && strings.EqualFold(tx.Data, "Mint")) {
				if balance.Balance > tx.Asset.Value {
					balance.Balance -= tx.Asset.Value
				} else {
					balance.Balance = 0
				}
				senderAccount.EBalances[symbol][tx.Asset.ExternalSenderAddress] = balance
			}

			senderAccount.Nonce = tx.Asset.Nonce

			sactbz, err := cdc.MarshalJSON(senderAccount)
			if err != nil {
				log.Printf("Failed to Marshal sender's account: %v", err)
				plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
				continue
			}
			addressBytes := []byte(pubKey.GetAddress())
			err = stateTrie.TryUpdate(addressBytes, sactbz)
			if err != nil {
				log.Printf("Failed to store account in state db: %v", err)
				plog.Error().Msgf("Failed to store account in state db: %v", err)
				tx.Status = "failed"
				txbz, err = cdc.MarshalJSON(&tx)
				(*txs)[i] = txbz
				if err != nil {
					log.Printf("Failed to encode failed tx: %v", err)
					plog.Error().Msgf("Failed to encode failed tx: %v", err)
				}
			}
			tx.Status = "success"
			txbz, err = cdc.MarshalJSON(&tx)
			txStr.Status = tx.Status
			txlist.Transactions = append(txlist.Transactions, &txStr)
			(*txs)[i] = txbz
			if err != nil {
				log.Printf("Failed to encode failed tx: %v", err)
				plog.Error().Msgf("Failed to encode failed tx: %v", err)
			}

			continue

		} else if strings.EqualFold(tx.Type, "Update") ||
			strings.EqualFold(tx.Type, "Register") ||
			strings.EqualFold(tx.Type, "Lock") ||
			strings.EqualFold(tx.Type, "Redeem") {

			switch txType := strings.ToUpper(tx.Type); txType {
			case "REGISTER":
				senderAccount = *(registerAccount(&senderAccount, &tx))
			case "UPDATE":
				senderAccount = *(updateAccount(&senderAccount, &tx))
			case "LOCK":
				senderAccount = *(updateAccountLockedBalance(&senderAccount, &tx))
			case "REDEEM":
				senderAccount = *(updateRedeemAccountLockedBalance(&senderAccount, &tx))
			}

			sactbz, err := cdc.MarshalJSON(senderAccount)
			if err != nil {
				log.Printf("Failed to Marshal sender's account: %v", err)
				plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
				continue
			}
			addressBytes := []byte(pubKey.GetAddress())
			err = stateTrie.TryUpdate(addressBytes, sactbz)
			if err != nil {
				log.Printf("Failed to store account in state db: %v", err)
				plog.Error().Msgf("Failed to store account in state db: %v", err)
				tx.Status = "failed"
				txbz, err = cdc.MarshalJSON(&tx)
				(*txs)[i] = txbz
				txStr.Status = tx.Status
				txlist.Transactions = append(txlist.Transactions, &txStr)
				if err != nil {
					log.Printf("Failed to encode failed tx: %v", err)
					plog.Error().Msgf("Failed to encode failed tx: %v", err)
				}
			}
			tx.Status = "success"
			txbz, err = cdc.MarshalJSON(&tx)
			(*txs)[i] = txbz
			txStr.Status = tx.Status
			txlist.Transactions = append(txlist.Transactions, &txStr)
			if err != nil {
				log.Printf("Failed to encode failed tx: %v", err)
				plog.Error().Msgf("Failed to encode failed tx: %v", err)
			}
			continue
		}

		if strings.EqualFold(tx.Asset.Network, "Herdius") {

			// Verify if sender has an address for corresponding external asset
			if !strings.EqualFold(tx.Asset.Symbol, aSymbol.HER) &&
				!isExternalAssetAddressExist(&senderAccount, tx.Asset.Symbol, tx.Asset.ExternalSenderAddress) {
				tx.Status = "failed"
				txbz, err = cdc.MarshalJSON(&tx)
				(*txs)[i] = txbz
				txStr.Status = tx.Status
				txlist.Transactions = append(txlist.Transactions, &txStr)
				if err != nil {
					log.Printf("Failed to encode failed tx: %v", err)
					plog.Error().Msgf("Failed to encode failed tx: %v", err)
				}
				continue
			}

			// Get Reciever's Account
			rcvrAddressBytes := []byte(tx.RecieverAddress)
			rcvrActbz, _ := stateTrie.TryGet(rcvrAddressBytes)

			var rcvrAccount statedb.Account

			err = cdc.UnmarshalJSON(rcvrActbz, &rcvrAccount)

			if err != nil {
				log.Printf("Failed to Unmarshal receiver's account: %v", err)
				plog.Error().Msgf("Failed to Unmarshal receiver's account: %v", err)
				continue
			}

			// Verify if Receiver has an address for corresponding external asset
			if !strings.EqualFold(tx.Asset.Symbol, aSymbol.HER) && len(rcvrAccount.EBalances[tx.Asset.Symbol]) == 0 {
				tx.Status = "failed"
				txbz, err = cdc.MarshalJSON(&tx)
				(*txs)[i] = txbz
				txStr.Status = tx.Status
				txlist.Transactions = append(txlist.Transactions, &txStr)
				if err != nil {
					log.Printf("Failed to encode failed tx: %v", err)
					plog.Error().Msgf("Failed to encode failed tx: %v", err)
				}
				continue
			}

			// TODO: Deduct Fee from Sender's Account when HER Fee is applied
			//Withdraw fund from Sender Account
			withdraw(&senderAccount, tx.Asset.Symbol, tx.Asset.ExternalSenderAddress, tx.Asset.Value)

			// Credit Reciever's Account
			// If credit to external address, pick first account
			// TODO: Should we consider tx.Asset.ExternalRecieverAddress?
			deposit(&rcvrAccount, tx.Asset.Symbol, rcvrAccount.FirstExternalAddress[tx.Asset.Symbol], tx.Asset.Value)

			senderAccount.Nonce = tx.Asset.Nonce
			updatedSenderAccount, err := cdc.MarshalJSON(senderAccount)
			if err != nil {
				log.Printf("Failed to Marshal sender's account: %v", err)
				plog.Error().Msgf("Failed to Marshal sender's account: %v", err)
			}

			err = stateTrie.TryUpdate(senderAddressBytes, updatedSenderAccount)
			if err != nil {
				log.Printf("Failed to update sender's account in state db: %v", err)
				plog.Error().Msgf("Failed to update sender's account in state db: %v", err)
			}

			updatedRcvrAccount, err := cdc.MarshalJSON(rcvrAccount)

			if err != nil {
				log.Printf("Failed to Marshal receiver's account: %v", err)
				plog.Error().Msgf("Failed to Marshal receiver's account: %v", err)
			}

			err = stateTrie.TryUpdate(rcvrAddressBytes, updatedRcvrAccount)
			if err != nil {
				log.Printf("Failed to update receiver's account in state db: %v", err)
				plog.Error().Msgf("Failed to update receiver's account in state db: %v", err)
			}

			// TODO: Fee should be credit to intended recipient
		}

		// Mark the tx as success and
		// add the updated tx to batch that will finally be added to singular block
		tx.Status = "success"
		txbz, err = cdc.MarshalJSON(&tx)
		(*txs)[i] = txbz
		txStr.Status = tx.Status
		txlist.Transactions = append(txlist.Transactions, &txStr)
		if err != nil {
			log.Printf("Failed to encode failed tx: %v", err)
			plog.Error().Msgf("Failed to encode failed tx: %v", err)
		}

	}

	root, err := stateTrie.Commit(nil)
	if err != nil {
		log.Println("Failed to commit to state trie:", err)
		plog.Error().Msgf("Failed to commit to state trie: %v", err)
	}
	s.SetStateRoot(root)
	return txlist, nil
}
