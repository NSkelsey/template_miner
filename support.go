// These functions have been shamelessly taken from https://github.com/btcsuite/btcd/blob/master/mining.go

package support

import (
	"math/rand"

	"github.com/PointCoin/btcnet"
	"github.com/PointCoin/btcutil"
	"github.com/PointCoin/btcwire"
	"github.com/PointCoin/pointcoind/blockchain"
	"github.com/PointCoin/pointcoind/txscript"
)

var coinbaseFlags = "/P2SH/btcd/"

// standardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks and adds
// the extra nonce as well as additional coinbase flags.
func standardCoinbaseScript(nextBlockHeight int64, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(nextBlockHeight).
		AddUint64(extraNonce).AddData([]byte(coinbaseFlags)).Script()
}

// createCoinbaseTx returns a coinbase transaction paying an appropriate subsidy
// based on the passed block height to the provided address.  When the address
// is nil, the coinbase transaction will instead be redeemable by anyone.
//
// See the comment for NewBlockTemplate for more information about why the nil
// address handling is useful.
func createCoinbaseTx(coinbaseScript []byte, nextBlockHeight int64, addr btcutil.Address) (*btcutil.Tx, error) {
	// Create the script to pay to the provided payment address if one was
	// specified.  Otherwise create a script that allows the coinbase to be
	// redeemable by anyone.
	var pkScript []byte
	if addr != nil {
		var err error
		pkScript, err = txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		scriptBuilder := txscript.NewScriptBuilder()
		pkScript, err = scriptBuilder.AddOp(txscript.OP_TRUE).Script()
		if err != nil {
			return nil, err
		}
	}

	tx := btcwire.NewMsgTx()
	tx.AddTxIn(&btcwire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *btcwire.NewOutPoint(&btcwire.ShaHash{},
			btcwire.MaxPrevOutIndex),
		SignatureScript: coinbaseScript,
		Sequence:        btcwire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&btcwire.TxOut{
		Value: blockchain.CalcBlockSubsidy(nextBlockHeight,
			&btcnet.MainNetParams),
		PkScript: pkScript,
	})
	return btcutil.NewTx(tx), nil
}

func CreateCoinbaseTx(nextBlockHeight int64, addr btcutil.Address) (*btcutil.Tx, error) {
	n := uint64(rand.Uint32())
	script, err := standardCoinbaseScript(nextBlockHeight, n)
	if err != nil {
		return nil, err
	}
	return createCoinbaseTx(script, nextBlockHeight, addr)
}