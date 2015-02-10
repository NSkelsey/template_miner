package main

import (
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/PointCoin/btcjson"
	"github.com/PointCoin/btcnet"
	"github.com/PointCoin/btcrpcclient"
	"github.com/PointCoin/btcutil"
	"github.com/PointCoin/btcwire"
	"github.com/PointCoin/pointcoind/blockchain"

	"github.com/PointCoin/support"
)

const (
	rpcuser = "rpc"
	rpcpass = "supasecretpassword"
	cert    = "/home/ubuntu/.pointcoind/rpc.cert"

	logUpdateSecs    = 5
	refreshBlockSecs = 15
)

func main() {

	// Setup the client using application constants, die horribly if there's a problem
	client := setupRpcClient()

	// Declare timers to use in the select
	logOutputTimer := time.NewTicker(time.Second * logUpdateSecs)
	refreshTimer := time.NewTicker(time.Second * refreshBlockSecs)

	// Declare variables to use in our main loop
	var template *btcjson.GetBlockTemplateResult
	var block *btcwire.MsgBlock
	var difficulty *big.Int
	var hashCounter

	// Get a new block template from pointcoind.
	template, err = client.GetBlockTemplate(&btcjson.TemplateRequest{})
	if err != nil {
		log.Fatal(err)
	}

	// Create the header that we will mine.
	block, err = createBlock(template)
	if err != nil {
		log.Fatal(err)
	}

	// Determine the current network difficulty.
	difficulty = formatDiff(template.Bits)

	for {
		select {
		case <-logOuputTimer.C:
			logHashRate(logUpdateSecs, hashCounter)
			hashCounter = 0

		case <-refreshTimer.C:
			// Every 15 seconds get a new block template from pointcoind,
			// create a new block header to mine off of and set the new
			// difficulty

			// Get a new block template from pointcoind.
			template, err = client.GetBlockTemplate(&btcjson.TemplateRequest{})
			if err != nil {
				log.Fatal(err)
			}

			block, err = createBlock(template)
			if err != nil {
				log.Fatal(err)
			}

			difficulty = formatDiff(template.Bits)

		default:
			// Non-blocking case to fall through
		}

		// Increment the nonce in the block's header. It might overflow, but that's
		// no big deal.
		block.Header.Nonce += 1

		// Hash the header
		hash, _ := block.Header.BlockSha()
		hashCounter += 1

		if lessThanDiff(hash, difficulty) {
			// Success! Send the whole block

			// We use a btcutil block b/c SubmitBlock demands it.
			err := client.SubmitBlock(btcutil.NewBlock(block), nil)
			if err != nil {
				log.Errorf("Block Submission to node failed with: %s\n", err)
			}

			log.Printf("Block Submitted! Hash:[%s]\n", hash)
		}

	}
}

// setupRpcClient handles establishing a connection to the pointcoind using
// the provided parameters. The function will throw an error if the full node
// is not running.
func setupRpcClient() *btcrpcclient.Client {
	// Get the raw bytes of the certificate required by the rpcclient.
	cert, err := ioutil.ReadFile(cert)
	if err != nil {
		log.Fatal(err)
	}

	// Setup the RPC client
	connCfg := &btcrpcclient.ConnConfig{
		Host:         "127.0.0.1:8334",
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

	// Test the connection to see if we can really connect
	_, err := client.GetInfo()
	if err != nil {
		log.Fatal(err)
	}

	return client
}

// logHashRate takes the current log timer's window and produces the rate at which
// it has been mining for the last [time window] seconds.
func logHashRate(windowsecs int, hashes int) {
	hashesPerSec := float64(hashes) / float64(windowsecs)
	log.Printf("Hashing speed: %6.0f kilohashes/s", hashesPerSec/1000)
}

// lessThanDiff returns true if the hash satisifies the target difficulty. That
// is to say if the hash interpreted as a big integer is less than the required
// difficulty then return true otherwise return false.
func lessThanDiff(hash *btcwire.ShaHash, difficulty *big.Int) bool {
	bigI := blockchain.ShaHashToBig(&hash)
	return bigI.ShaHashToBig(&hash).Cmp(targetDifficulty) <= 0
}

// foramtDiff converts the current blockchain difficulty from the format provided
// by a Block Template response to a big integer for use in comparisons
func formatDiff(bits string) big.Int {
	// Convert the difficulty bits into a unint32.
	b, err := strconv.ParseUint(template.Bits, 16, 32)
	if err != nil {
		log.Fatal(err) // This should not fail, so die horribly if it does
	}

	// Then into a big.Int
	return &blockchain.CompactToBig(b)
}

// createBlock creates a new block from the provided block template. The majority
// of the work here is interpreting the information provided by the block template.
func createBlock(template *btcjson.GetBlockTemplateResult) (*btcwire.MsgBlock, error) {

	// TODO handle addr
	// A hardcoded address to send mined coins to.
	a := "PsVSrUSQf72X6GWFQXJPxR7WSAPVRb1gWx"
	addr, err := btcutil.DecodeAddress(a, &btcnet.MainNetParams)

	// Use supporting functions to create a CoinbaseTx
	coinbaseTx, err := support.CreateCoinbaseTx(template.Height+1, addr)
	if err != nil {
		return nil, err
	}

	txs := []*btcutil.Tx{coinbaseTx}
	// The last element in the array is the root.
	store := blockchain.BuildMerkleTreeStore(txs)
	// Create a merkleroot from a list of 1 transaction.
	merkleRoot := store[len(store)-1]

	// Convert the difficulty bits into a uint64
	d, _ := strconv.ParseUint(template.Bits, 16, 32)

	prevH, _ := btcwire.NewShaHashFromStr(template.PreviousHash)
	startNonce := rand.Uint32()

	header := btcwire.NewBlockHeader(prevH, merkleRoot, uint32(d), startNonce)

	msgBlock := btcwire.NewMsgBlock(header)
	msgBlock.AddTransaction(coinbaseTx.MsgTx())

	return msgBlock, nil
}
