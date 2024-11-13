package tron

import (
	"context"
	"encoding/json"
	ethclient2 "github.com/ava-labs/coreth/ethclient"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/glog"
	"github.com/juju/errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthereumClient wraps a client to implement the EVMClient interface
type EthereumClient struct {
	*ethclient.Client
}

type rpcTransaction struct {
	tx *types.Transaction
	txExtraInfo
}

type txExtraInfo struct {
	BlockNumber *string         `json:"blockNumber,omitempty"`
	BlockHash   *common.Hash    `json:"blockHash,omitempty"`
	From        *common.Address `json:"from,omitempty"`
}

func (tx *rpcTransaction) UnmarshalJSON(msg []byte) error {
	var raw *TransactionRaw
	if err := json.Unmarshal(msg, &raw); err != nil {
		glog.Error("1111.UnmarshalJSON.err: ", err)
		return err
	}

	jsonStr, _ := json.Marshal(TransactionRaw2{
		BlockHash:        raw.BlockHash,
		BlockNumber:      raw.BlockNumber,
		TransactionIndex: uint64(raw.TransactionIndex),
		Hash:             raw.Hash,
		From:             raw.From,
		To:               raw.To,
		Value:            raw.Value,
		Input:            raw.Input,
		Nonce:            hexutil.Uint64(raw.Nonce.Uint64()),
		R:                (*hexutil.Big)(raw.R.Big()),
		S:                (*hexutil.Big)(raw.S.Big()),
		V:                raw.V.String(),
		Type:             raw.Type,
		Gas:              raw.Gas,
		GasPrice:         raw.GasPrice,
	})

	if err2 := json.Unmarshal(jsonStr, &tx.tx); err2 != nil {
		glog.Error("333.UnmarshalJSON.err: ", err2)
		return err2
	}
	if err3 := json.Unmarshal(jsonStr, &tx.txExtraInfo); err3 != nil {
		glog.Error("444.UnmarshalJSON.err: ", err3)
		return err3
	}
	return nil
	//return json.Unmarshal(jsonStr, &tx.txExtraInfo)
}

type TransactionRaw struct {
	BlockHash        common.Hash      `json:"blockHash"`
	BlockNumber      hexutil.Uint64   `json:"blockNumber"`
	TransactionIndex hexutil.Uint64   `json:"transactionIndex"`
	Hash             common.Hash      `json:"hash"`
	From             common.Address   `json:"from"`
	To               common.Address   `json:"to"`
	Value            hexutil.Big      `json:"value"`
	Input            string           `json:"input"`
	Nonce            types.BlockNonce `json:"nonce"`
	R                common.Hash      `json:"r"`
	S                common.Hash      `json:"s"`
	V                hexutil.Uint64   `json:"v"`
	Type             hexutil.Uint64   `json:"type"`
	Gas              hexutil.Uint64   `json:"gas"`
	GasPrice         hexutil.Uint64   `json:"gasPrice"`
}

type TransactionRaw2 struct {
	BlockHash        common.Hash    `json:"blockHash"`
	BlockNumber      hexutil.Uint64 `json:"blockNumber"`
	TransactionIndex uint64         `json:"transactionIndex"`
	Hash             common.Hash    `json:"hash"`
	From             common.Address `json:"from"`
	To               common.Address `json:"to"`
	Value            hexutil.Big    `json:"value"`
	Input            string         `json:"input"`
	Nonce            hexutil.Uint64 `json:"nonce"`
	R                *hexutil.Big   `json:"r"`
	S                *hexutil.Big   `json:"s"`
	V                string         `json:"v"`
	Type             hexutil.Uint64 `json:"type"`
	Gas              hexutil.Uint64 `json:"gas"`
	GasPrice         hexutil.Uint64 `json:"gasPrice"`
}

type rpcBlock struct {
	Hash         common.Hash      `json:"hash"`
	Transactions []rpcTransaction `json:"transactions"`
}

type Header struct {
	ParentHash  common.Hash      `json:"parentHash"       gencodec:"required"`
	UncleHash   common.Hash      `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address   `json:"miner"`
	TxHash      common.Hash      `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash      `json:"receiptsRoot"     gencodec:"required"`
	Bloom       types.Bloom      `json:"logsBloom"        gencodec:"required"`
	Difficulty  hexutil.Uint64   `json:"difficulty"       gencodec:"required"`
	Number      hexutil.Uint64   `json:"number"           gencodec:"required"`
	GasLimit    hexutil.Uint64   `json:"gasLimit"         gencodec:"required"`
	GasUsed     hexutil.Uint64   `json:"gasUsed"          gencodec:"required"`
	Time        hexutil.Uint64   `json:"timestamp"        gencodec:"required"`
	MixDigest   common.Hash      `json:"mixHash"`
	Nonce       types.BlockNonce `json:"nonce"`
	BaseFee     hexutil.Uint64   `json:"baseFeePerGas" rlp:"optional"`
}

// BlockByHash returns the given full block.
//
// Note that loading full blocks requires two requests. Use HeaderByHash
// if you don't need all transactions or uncle headers.
func (ec *EthereumClient) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return ec.getBlock(ctx, "eth_getBlockByHash", hash, true)
}

// BlockByNumber returns a block from the current canonical chain. If number is nil, the
// latest known block is returned.
//
// Note that loading full blocks requires two requests. Use HeaderByNumber
// if you don't need all transactions or uncle headers.
func (ec *EthereumClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return ec.getBlock(ctx, "eth_getBlockByNumber", ethclient2.ToBlockNumArg(number), true)
}

func (ec *EthereumClient) getBlock(ctx context.Context, method string, args ...interface{}) (*types.Block, error) {
	var raw json.RawMessage
	err := ec.Client.Client().CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	}

	// Decode header and transactions.
	var head *Header
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	// When the block is not found, the API returns JSON null.
	if head == nil {
		return nil, ethereum.NotFound
	}

	var body rpcBlock
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.Errorf("333.getBlock err=%v...", err)
	}
	if head.TxHash == types.EmptyTxsHash && len(body.Transactions) > 0 {
		return nil, errors.New("server returned non-empty transaction list but block header indicates no transactions")
	}
	if head.TxHash != types.EmptyTxsHash && len(body.Transactions) == 0 {
		return nil, errors.New("server returned empty transaction list but block header indicates transactions")
	}

	var uncles []*types.Header

	// Fill the sender cache of transactions in the block.
	txs := make([]*types.Transaction, len(body.Transactions))
	for i, tx := range body.Transactions {
		if tx.From != nil {
			setSenderFromServer(tx.tx, *tx.From, body.Hash)
		}
		txs[i] = tx.tx
	}

	return types.NewBlockWithHeader(&types.Header{
		ParentHash:       head.ParentHash,
		UncleHash:        head.UncleHash,
		Coinbase:         head.Coinbase,
		Root:             common.Hash{},
		TxHash:           head.TxHash,
		ReceiptHash:      head.ReceiptHash,
		Bloom:            head.Bloom,
		Difficulty:       big.NewInt(int64(head.Difficulty)),
		Number:           big.NewInt(int64(head.Number)),
		GasLimit:         uint64(head.GasLimit),
		GasUsed:          uint64(head.GasUsed),
		Time:             uint64(head.Time),
		Extra:            nil,
		MixDigest:        head.MixDigest,
		Nonce:            head.Nonce,
		BaseFee:          big.NewInt(int64(head.BaseFee)),
		WithdrawalsHash:  nil,
		BlobGasUsed:      nil,
		ExcessBlobGas:    nil,
		ParentBeaconRoot: nil,
	}).WithBody(txs, uncles), nil
}

// HeaderByHash returns the block header with the given hash.
func (ec *EthereumClient) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	var head *Header
	err := ec.Client.Client().CallContext(ctx, &head, "eth_getBlockByHash", hash, false)
	if err == nil && head == nil {
		err = ethereum.NotFound
	}
	return &types.Header{
		ParentHash:       head.ParentHash,
		UncleHash:        head.UncleHash,
		Coinbase:         head.Coinbase,
		Root:             common.Hash{},
		TxHash:           head.TxHash,
		ReceiptHash:      head.ReceiptHash,
		Bloom:            head.Bloom,
		Difficulty:       big.NewInt(int64(head.Difficulty)),
		Number:           big.NewInt(int64(head.Number)),
		GasLimit:         uint64(head.GasLimit),
		GasUsed:          uint64(head.GasUsed),
		Time:             uint64(head.Time),
		Extra:            nil,
		MixDigest:        head.MixDigest,
		Nonce:            head.Nonce,
		BaseFee:          big.NewInt(int64(head.BaseFee)),
		WithdrawalsHash:  nil,
		BlobGasUsed:      nil,
		ExcessBlobGas:    nil,
		ParentBeaconRoot: nil,
	}, err
}

// HeaderByNumber2 returns a block header from the current canonical chain. If number is
// nil, the latest known header is returned.
func (ec *EthereumClient) HeaderByNumber2(ctx context.Context, number *big.Int) (*types.Header, error) {
	var head *Header
	err := ec.Client.Client().CallContext(ctx, &head, "eth_getBlockByNumber", ethclient2.ToBlockNumArg(number), false)
	if err == nil && head == nil {
		err = ethereum.NotFound
	}
	return &types.Header{
		ParentHash:       head.ParentHash,
		UncleHash:        head.UncleHash,
		Coinbase:         head.Coinbase,
		Root:             common.Hash{},
		TxHash:           head.TxHash,
		ReceiptHash:      head.ReceiptHash,
		Bloom:            head.Bloom,
		Difficulty:       big.NewInt(int64(head.Difficulty)),
		Number:           big.NewInt(int64(head.Number)),
		GasLimit:         uint64(head.GasLimit),
		GasUsed:          uint64(head.GasUsed),
		Time:             uint64(head.Time),
		Extra:            nil,
		MixDigest:        head.MixDigest,
		Nonce:            head.Nonce,
		BaseFee:          big.NewInt(int64(head.BaseFee)),
		WithdrawalsHash:  nil,
		BlobGasUsed:      nil,
		ExcessBlobGas:    nil,
		ParentBeaconRoot: nil,
	}, err
}

// ----

// TransactionByHash returns the transaction with the given hash.
//func (ec *EthereumClient) TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error) {
//	//var transaction *rpcTransaction
//	//var raw json.RawMessage
//	var raw TransactionRaw
//	err = ec.Client.Client().CallContext(ctx, &raw, "eth_getTransactionByHash", hash)
//	if err != nil {
//		return nil, false, err
//	}
//	//glog.Error("111.TransactionByHash", raw)
//
//	jsonStr, _ := json.Marshal(TransactionRaw2{
//		BlockHash:        raw.BlockHash,
//		BlockNumber:      raw.BlockNumber,
//		TransactionIndex: uint64(raw.TransactionIndex),
//		Hash:             raw.Hash,
//		From:             raw.From,
//		To:               raw.To,
//		Value:            raw.Value,
//		Input:            raw.Input,
//		Nonce:            hexutil.Uint64(raw.Nonce.Uint64()),
//		R:                raw.R,
//		S:                raw.S,
//		V:                raw.V.String(),
//		Type:             raw.Type,
//		Gas:              raw.Gas,
//		GasPrice:         raw.GasPrice,
//	})
//	//glog.Error("222.TransactionByHash", string(jsonStr))
//
//	var transaction *types.Transaction
//	if err := json.Unmarshal(jsonStr, &transaction); err != nil {
//		return nil, false, err
//	}
//	setSenderFromServer(transaction, raw.From, raw.BlockHash)
//
//	//glog.Error("444.TransactionByHash:hash: ", transaction.Hash())
//	//glog.Error("444.TransactionByHash:gas: ", transaction.Gas())
//	//glog.Error("444.TransactionByHash.GasPrice: ", transaction.GasPrice())
//	//glog.Error("444.TransactionByHash.to: ", transaction.To())
//	//glog.Error("444.TransactionByHash.value: ", transaction.Value())
//	//glog.Error("444.TransactionByHash.time: ", transaction.Time().String())
//
//	txString, _ := transaction.MarshalJSON()
//	glog.Error("555.TransactionByHash", string(txString))
//
//	// error: cannot unmarshal hex number with leading zero digits into Go struct field txJSON.nonce of type hexutil.Uint64
//	// todo: isPending = json.BlockNumber == nil
//	return transaction, false, nil
//}
