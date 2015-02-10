package main

import (
	"io/ioutil"
	"log"
	"strconv"

	"github.com/PointCoin/btcjson"
	"github.com/PointCoin/btcnet"
	"github.com/PointCoin/btcrpcclient"
	"github.com/PointCoin/btcutil"
	"github.com/PointCoin/btcwire"
	"github.com/PointCoin/pointcoind/blockchain"
	"github.com/davecgh/go-spew/spew"

	"github.com/PointCoin/support"
)

const (
	host    = "127.0.0.1:8334"
	rpcuser = "rpc"
	rpcpass = "supasecretpassword"
	cert    = "/home/ubuntu/.pointcoind/rpc.cert"
	id      = 1
)

func main() {

	// Get the raw bytes of the certificate required by the rpcclient.
	cert, err := ioutil.ReadFile(cert)
	if err != nil {
		log.Fatal(err)
	}

	// Setup the RPC client
	connCfg := &btcrpcclient.ConnConfig{
		Host:         host,
		User:         rpcuser,
		Pass:         rpcpass,
		Certificates: cert,
		// Use the websocket endpoint to keep the connection alive
		// in the event we want to do polling.
		Endpoint: "ws",
	}

	client, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown()

	// Get a block template from pointcoind.
	template, err := client.GetBlockTemplate(&btcjson.TemplateRequest{})
	if err != nil {
		log.Fatal(err)
	}

	// A hardcoded address to send mined coins to.
	a := "PsVSrUSQf72X6GWFQXJPxR7WSAPVRb1gWx"
	addr, err := btcutil.DecodeAddress(a, &btcnet.MainNetParams)

	// Use supporting functions to create a CoinbaseTx
	coinbaseTx, err := support.CreateCoinbaseTx(template.Height+1, addr)
	if err != nil {
		log.Fatal(err)
	}

	txs := []*btcutil.Tx{coinbaseTx}
	// The last element in the array is the root.
	store := blockchain.BuildMerkleTreeStore(txs)
	// Create a merkleroot from a list of 1 transaction.
	merkleRoot := store[len(store)-1]

	// Convert the difficulty bits into a unint32.
	d, err := strconv.ParseUint(template.Bits, 16, 32)
	if err != nil {
		log.Fatal(err)
	}

	prevH, _ := btcwire.NewShaHashFromStr(template.PreviousHash)

	header := btcwire.NewBlockHeader(prevH, merkleRoot, uint32(d), 0)
	// TODO(askuck) Search for a hash of the header and track the number of iterations
	// This block of code should be your model. Look at how that timer works.
	// We just sit here and churn.
	// https://github.com/btcsuite/btcd/blob/master/cpuminer.go#L200-L245

	maxNonce := ^uint32(0)
	targetDifficulty := blockchain.CompactToBig(header.Bits)

	for i := uint32(0); i <= maxNonce; i++ {

		header.Nonce = i
		hash, _ := header.BlockSha()

		if blockchain.ShaHashToBig(&hash).Cmp(targetDifficulty) <= 0 {
			break
		}
	}

	msgBlock := btcwire.NewMsgBlock(header)
	msgBlock.AddTransaction(coinbaseTx.MsgTx())

	// Submit the Block to the network.
	b := btcutil.NewBlock(msgBlock)
	r := client.SubmitBlock(b, nil)

	spew.Dump(r)
}