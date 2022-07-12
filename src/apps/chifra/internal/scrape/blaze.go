package scrapePkg

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/config"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/file"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/rpcClient"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/tslib"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/utils"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/validate"
)

// ScrapedData combines the block data, trace data, and log data into a single structure
type ScrapedData struct {
	blockNumber int
	traces      rpcClient.Traces
	logs        rpcClient.Logs
}

func (opts *ScrapeOptions) HandleScrapeBlaze() error {

	meta, _ := rpcClient.GetMetaData(opts.Globals.Chain, opts.Globals.TestMode)

	rpcProvider := config.GetRpcProvider(opts.Globals.Chain)
	fmt.Println(opts)

	blockChannel := make(chan int)
	addressChannel := make(chan ScrapedData)
	tsChannel := make(chan tslib.Timestamp)

	var blockWG sync.WaitGroup
	blockWG.Add(int(opts.BlockChanCnt))
	for i := 0; i < int(opts.BlockChanCnt); i++ {
		go opts.Z_blaze_processBlocks(meta, rpcProvider, blockChannel, addressChannel, tsChannel, &blockWG)
	}

	var addressWG sync.WaitGroup
	addressWG.Add(int(opts.AddrChanCnt))
	for i := 0; i < int(opts.AddrChanCnt); i++ {
		go opts.Z_blaze_processAddresses(meta, rpcProvider, addressChannel, &addressWG)
	}

	// TODO: BOGUS IS USING THIS FILE THE BEST WAY - IS THIS A GOOD FILENAME
	tsFilename := config.GetPathToCache(opts.Globals.Chain) + "tmp/tempTsFile.txt"
	tsFile, err := os.Create(tsFilename)
	if err != nil {
		log.Fatalf("Unable to create file: %v", err)
	}
	defer func() {
		tsFile.Close()
		file.Copy(tsFilename, "./file.save")
	}()

	var tsWG sync.WaitGroup
	tsWG.Add(int(opts.AddrChanCnt))
	for i := 0; i < int(opts.AddrChanCnt); i++ {
		go opts.Z_blaze_processTimestamps(rpcProvider, tsChannel, tsFile, &tsWG)
	}

	for block := int(opts.StartBlock); block < int(opts.StartBlock+opts.BlockCnt); block++ {
		blockChannel <- block
	}

	close(blockChannel)
	blockWG.Wait()

	close(addressChannel)
	addressWG.Wait()

	close(tsChannel)
	tsWG.Wait()

	return nil
}

// processBlocks Process the block channel and for each block query the node for both traces and logs. Send results to addressChannel
func (opts *ScrapeOptions) Z_blaze_processBlocks(meta *rpcClient.MetaData, rpcProvider string, blockChannel chan int, addressChannel chan ScrapedData, tsChannel chan tslib.Timestamp, blockWG *sync.WaitGroup) {
	for blockNum := range blockChannel {

		// RPCPayload is used during to make calls to the RPC.
		var traces rpcClient.Traces
		tracePayload := rpcClient.RPCPayload{
			Jsonrpc:   "2.0",
			Method:    "trace_block",
			RPCParams: rpcClient.RPCParams{fmt.Sprintf("0x%x", blockNum)},
			ID:        1002,
		}
		err := rpcClient.FromRpc(rpcProvider, &tracePayload, &traces)
		if err != nil {
			// TODO: BOGUS - RETURN VALUE FROM BLAZE
			fmt.Println("FromRpc(traces) returned error", err)
			os.Exit(1)
		}

		var logs rpcClient.Logs
		logsPayload := rpcClient.RPCPayload{
			Jsonrpc:   "2.0",
			Method:    "eth_getLogs",
			RPCParams: rpcClient.RPCParams{rpcClient.LogFilter{Fromblock: fmt.Sprintf("0x%x", blockNum), Toblock: fmt.Sprintf("0x%x", blockNum)}},
			ID:        1003,
		}
		err = rpcClient.FromRpc(rpcProvider, &logsPayload, &logs)
		if err != nil {
			// TODO: BOGUS - RETURN VALUE FROM BLAZE
			fmt.Println("FromRpc(logs) returned error", err)
			os.Exit(1)
		}

		addressChannel <- ScrapedData{
			blockNumber: blockNum,
			traces:      traces,
			logs:        logs,
		}

		// TODO: BOGUS The timeStamp value is not used here (the Consolidation Loop calls into
		// TODO: BOGUS the same routine again). We should use this value here and remove the
		// TODO: BOGUS call from the Consolidation Loop
		tsChannel <- tslib.Timestamp{
			Ts: uint32(rpcClient.GetBlockTimestamp(rpcProvider, uint64(blockNum))),
			Bn: uint32(blockNum),
		}
	}

	blockWG.Done()
}

func (opts *ScrapeOptions) Z_blaze_processAddresses(meta *rpcClient.MetaData, rpcProvider string, addressChannel chan ScrapedData, addressWG *sync.WaitGroup) {
	for sData := range addressChannel {
		addressMap := make(map[string]bool)
		opts.Z_blaze_extractFromTraces(rpcProvider, sData.blockNumber, &sData.traces, addressMap)
		opts.Z_blaze_extractFromLogs(sData.blockNumber, &sData.logs, addressMap)
		opts.Z_blaze_writeAddresses(meta, sData.blockNumber, addressMap)
	}
	addressWG.Done()
}

var blazeMutex sync.Mutex

func (opts *ScrapeOptions) Z_blaze_processTimestamps(rpcProvider string, tsChannel chan tslib.Timestamp, tsFile *os.File, tsWg *sync.WaitGroup) {
	for ts := range tsChannel {
		blazeMutex.Lock()
		// TODO: BOGUS - THIS COULD EASILY WRITE TO AN ARRAY NOT A FILE
		fmt.Fprintf(tsFile, "%s-%s\n", utils.PadLeft(strconv.Itoa(int(ts.Bn)), 9), utils.PadLeft(strconv.Itoa(int(ts.Ts)), 9))
		blazeMutex.Unlock()
	}
	tsWg.Done()
}

func (opts *ScrapeOptions) Z_blaze_extractFromTraces(rpcProvider string, bn int, traces *rpcClient.Traces, addressMap map[string]bool) {
	if traces.Result == nil || len(traces.Result) == 0 {
		return
	}

	blockNumStr := utils.PadLeft(strconv.Itoa(bn), 9)
	for i := 0; i < len(traces.Result); i++ {

		idx := utils.PadLeft(strconv.Itoa(traces.Result[i].TransactionPosition), 5)
		blockAndIdx := "\t" + blockNumStr + "\t" + idx

		if traces.Result[i].Type == "call" {
			// If it's a call, get the to and from
			from := traces.Result[i].Action.From
			if goodAddr(from) {
				addressMap[from+blockAndIdx] = true
			}
			to := traces.Result[i].Action.To
			if goodAddr(to) {
				addressMap[to+blockAndIdx] = true
			}

		} else if traces.Result[i].Type == "reward" {
			if traces.Result[i].Action.RewardType == "block" {
				author := traces.Result[i].Action.Author
				if validate.IsZeroAddress(author) {
					// Early clients allowed misconfigured miner settings with address
					// 0x0 (reward got burned). We enter a false record with a false tx_id
					// to account for this.
					author = "0xdeaddeaddeaddeaddeaddeaddeaddeaddeaddead"
					addressMap[author+"\t"+blockNumStr+"\t"+"99997"] = true

				} else {
					if goodAddr(author) {
						addressMap[author+"\t"+blockNumStr+"\t"+"99999"] = true
					}
				}

			} else if traces.Result[i].Action.RewardType == "uncle" {
				author := traces.Result[i].Action.Author
				if validate.IsZeroAddress(author) {
					// Early clients allowed misconfigured miner settings with address
					// 0x0 (reward got burned). We enter a false record with a false tx_id
					// to account for this.
					author = "0xdeaddeaddeaddeaddeaddeaddeaddeaddeaddead"
					addressMap[author+"\t"+blockNumStr+"\t"+"99998"] = true

				} else {
					if goodAddr(author) {
						addressMap[author+"\t"+blockNumStr+"\t"+"99998"] = true
					}
				}

			} else if traces.Result[i].Action.RewardType == "external" {
				// This only happens in xDai as far as we know...
				author := traces.Result[i].Action.Author
				if goodAddr(author) {
					addressMap[author+"\t"+blockNumStr+"\t"+"99996"] = true
				}

			} else {
				fmt.Println("New type of reward", traces.Result[i].Action.RewardType)
			}

		} else if traces.Result[i].Type == "suicide" {
			// add the contract that died, and where it sent it's money
			address := traces.Result[i].Action.Address
			if goodAddr(address) {
				addressMap[address+blockAndIdx] = true
			}
			refundAddress := traces.Result[i].Action.RefundAddress
			if goodAddr(refundAddress) {
				addressMap[refundAddress+blockAndIdx] = true
			}

		} else if traces.Result[i].Type == "create" {
			// add the creator, and the new address name
			from := traces.Result[i].Action.From
			if goodAddr(from) {
				addressMap[from+blockAndIdx] = true
			}
			address := traces.Result[i].Result.Address
			if goodAddr(address) {
				addressMap[address+blockAndIdx] = true
			}

			// If it's a top level trace, then the call data is the init,
			// so to match with TrueBlocks, we just parse init
			if len(traces.Result[i].TraceAddress) == 0 {
				if len(traces.Result[i].Action.Init) > 10 {
					initData := traces.Result[i].Action.Init[10:]
					for i := 0; i < len(initData)/64; i++ {
						addr := string(initData[i*64 : (i+1)*64])
						if potentialAddress(addr) {
							addr = "0x" + string(addr[24:])
							if goodAddr(addr) {
								addressMap[addr+blockAndIdx] = true
							}
						}
					}
				}
			}

			// Handle contract creations that may have errored out
			if traces.Result[i].Action.To == "" {
				if traces.Result[i].Result.Address == "" {
					if traces.Result[i].Error != "" {
						var receipt rpcClient.Receipt
						var txReceiptPl = rpcClient.RPCPayload{
							Jsonrpc:   "2.0",
							Method:    "eth_getTransactionReceipt",
							RPCParams: rpcClient.RPCParams{traces.Result[i].TransactionHash},
							ID:        1005,
						}
						err := rpcClient.FromRpc(rpcProvider, &txReceiptPl, &receipt)
						if err != nil {
							// TODO: BOGUS - RETURN VALUE FROM BLAZE
							fmt.Println("FromRpc(transReceipt) returned error", err)
							os.Exit(1)
						}
						addr := receipt.Result.ContractAddress
						if goodAddr(addr) {
							addressMap[addr+blockAndIdx] = true
						}
					}
				}
			}

		} else {
			err := "New trace type:" + traces.Result[i].Type
			// TODO: BOGUS - RETURN VALUE FROM BLAZE
			fmt.Println("extractFromTraces -->", err)
			os.Exit(1)
		}

		// Try to get addresses from the input data
		if len(traces.Result[i].Action.Input) > 10 {
			inputData := traces.Result[i].Action.Input[10:]
			//fmt.Println("Input data:", inputData, len(inputData))
			for i := 0; i < len(inputData)/64; i++ {
				addr := string(inputData[i*64 : (i+1)*64])
				if potentialAddress(addr) {
					addr = "0x" + string(addr[24:])
					if goodAddr(addr) {
						addressMap[addr+blockAndIdx] = true
					}
				}
			}
		}

		// Parse output of trace
		if len(traces.Result[i].Result.Output) > 2 {
			outputData := traces.Result[i].Result.Output[2:]
			for i := 0; i < len(outputData)/64; i++ {
				addr := string(outputData[i*64 : (i+1)*64])
				if potentialAddress(addr) {
					addr = "0x" + string(addr[24:])
					if goodAddr(addr) {
						addressMap[addr+blockAndIdx] = true
					}
				}
			}
		}
	}
}

// extractFromLogs Extracts addresses from any part of the log data.
func (opts *ScrapeOptions) Z_blaze_extractFromLogs(bn int, logs *rpcClient.Logs, addressMap map[string]bool) {
	if logs.Result == nil || len(logs.Result) == 0 {
		return
	}

	blockNumStr := utils.PadLeft(strconv.Itoa(bn), 9)
	for i := 0; i < len(logs.Result); i++ {
		// Note: Maybe a bug? Does not process Log.Address (i.e., the emitter of the log)
		// Probably captured by the trace processing, but would be missed if we were
		// processing traces. Won't hurt to add (since the map removes dups) so it
		// should be added. Would be interested to test.

		idxInt, err := strconv.ParseInt(logs.Result[i].TransactionIndex, 0, 32)
		if err != nil {
			// TODO: BOGUS - RETURN VALUE FROM BLAZE
			fmt.Println("extractFromLogs --> strconv.ParseInt returned error", err)
			os.Exit(1)
		}
		idx := utils.PadLeft(strconv.FormatInt(idxInt, 10), 5)

		blockAndIdx := "\t" + blockNumStr + "\t" + idx

		for j := 0; j < len(logs.Result[i].Topics); j++ {
			addr := string(logs.Result[i].Topics[j][2:])
			if potentialAddress(addr) {
				addr = "0x" + string(addr[24:])
				if goodAddr(addr) {
					addressMap[addr+blockAndIdx] = true
				}
			}
		}

		if len(logs.Result[i].Data) > 2 {
			inputData := logs.Result[i].Data[2:]
			for i := 0; i < len(inputData)/64; i++ {
				addr := string(inputData[i*64 : (i+1)*64])
				if potentialAddress(addr) {
					addr = "0x" + string(addr[24:])
					if goodAddr(addr) {
						addressMap[addr+blockAndIdx] = true
					}
				}
			}
		}
	}
}

var nProcessed uint64 = 0

func (opts *ScrapeOptions) Z_blaze_writeAddresses(meta *rpcClient.MetaData, bn int, addressMap map[string]bool) {
	if len(addressMap) == 0 {
		return
	}

	blockNumStr := utils.PadLeft(strconv.Itoa(bn), 9)
	addressArray := make([]string, 0, len(addressMap))
	for record := range addressMap {
		addressArray = append(addressArray, record)
	}
	sort.Strings(addressArray)

	toWrite := []byte(strings.Join(addressArray[:], "\n") + "\n")

	bn, _ = strconv.Atoi(blockNumStr)
	fileName := config.GetPathToIndex(opts.Globals.Chain) + "ripe/" + blockNumStr + ".txt"

	ripeBlock := meta.Latest - utils.Min(meta.Latest, opts.UnripeDist)
	if bn > int(ripeBlock) {
		fileName = config.GetPathToIndex(opts.Globals.Chain) + "unripe/" + blockNumStr + ".txt"
	}

	err := ioutil.WriteFile(fileName, toWrite, 0744)
	if err != nil {
		// TODO: BOGUS - RETURN VALUE FROM BLAZE
		fmt.Println("writeAddresses --> ioutil.WriteFile returned error", err)
		os.Exit(1)
	}

	// TODO: BOGUS - TESTING SCRAPING
	// step := uint64(7)
	// if nProcessed%step == 0 {
	// 	dist := uint64(0)
	// 	if ripeBlock > uint64(bn) {
	// 		dist = (ripeBlock - uint64(bn))
	// 	}
	// 	f := "-------- ( ------)- <PROG>  : Scraping %-04d of %-04d at block %d of %d (%d blocks from head)\r"
	// 	fmt.Fprintf(os.Stderr, f, nProcessed, opts.BlockCnt, bn, ripeBlock, dist)
	// }
	nProcessed++
}

// goodAddr Returns true if the address is not a precompile and not the zero address
func goodAddr(addr string) bool {
	// As per EIP 1352, all addresses less or equal to the following value are reserved for pre-compiles.
	// We don't index precompiles. https://eips.ethereum.org/EIPS/eip-1352
	return addr > "0x000000000000000000000000000000000000ffff"
}

// potentialAddress processes a transaction's 'input' data and 'output' data or an event's data field. We call anything
// with 12 bytes of leading zeros but not more than 19 leading zeros (24 and 38 characters respectively).
func potentialAddress(addr string) bool {
	// Any 32-byte value smaller than this number is assumed to be a 'value'. We call them baddresses.
	// While this may seem like a lot of addresses being labeled as baddresses, it's not very many:
	// ---> 2 out of every 10000000000000000000000000000000000000000000000 are baddresses.
	small := "00000000000000000000000000000000000000ffffffffffffffffffffffffff"
	//        -------+-------+-------+-------+-------+-------+-------+-------+
	if addr <= small {
		return false
	}

	// Any 32-byte value with less than this many leading zeros is not an address (address are 20-bytes and
	// zero padded to the left)
	largePrefix := "000000000000000000000000"
	//              -------+-------+-------+
	if !strings.HasPrefix(addr, largePrefix) {
		return false
	}

	// Of the valid addresses, we assume any ending with this many trailing zeros is also a baddress.
	if strings.HasSuffix(addr, "00000000") {
		return false
	}
	return true
}

// TODO:
// TODO: This "baddress"
// TODO:
// TODO: 0x00000000000004ee2d6d415371e298f092cc0000
// TODO:
// TODO: appears in the index but it is not an actual address. It appears only four times in the entire index.
// TODO: We know this is not an address because it only appears the event 'data' section for Transfers or Approvals
// TODO: which we know to be the value, not an address.
// TODO:
// TODO: The trouble is knowing this is a "non-chain knowledge leak." The chain itself knows nothing about
// TODO: ERC20 tokens. I'm not sure how many 'false records' (or baddresses) this would remove, but it may
// TODO: be significant given that Transfers and Approvals dominate the chain data.
// TODO:
// TODO: What we could do is this:
// TODO:
// TODO: If we're scraping a log, and
// TODO:
// TODO: 	If we see certain topics (topic[0] is a Transfer or Approval, we do not include the value
// TODO:	even if it looks like an address. This is a very slippery slope. What does 'well known' mean?
// TODO:
// TODO: Another downside; implementing this would require a full re-generation of the index and would
// TODO: change the hashes and the underlying files. In order to do this, we would require a migration that
// TODO: removes the 'old' index from the end user's machine and then downloads the new index. We can do this,
// TODO: but it feels quite precarious.
// TODO:
// TODO: My expectation is that we will eventually have to re-generate the index at some point (version 1.0?).
// TODO: We can this then.
// TODO:
