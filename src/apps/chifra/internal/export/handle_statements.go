// Copyright 2021 The TrueBlocks Authors. All rights reserved.
// Use of this source code is governed by a license that can
// be found in the LICENSE file.

package exportPkg

import (
	"context"
	"fmt"
	"sort"

	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/base"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/filter"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/ledger"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/logger"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/monitor"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/names"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/output"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/types"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/utils"
)

func (opts *ExportOptions) HandleStatements(monitorArray []monitor.Monitor) error {
	chain := opts.Globals.Chain
	testMode := opts.Globals.TestMode
	filter := filter.NewFilter(
		opts.Reversed,
		opts.Reverted,
		opts.Fourbytes,
		base.BlockRange{First: opts.FirstBlock, Last: opts.LastBlock},
		base.RecordRange{First: opts.FirstRecord, Last: opts.GetMax()},
	)

	ctx, cancel := context.WithCancel(context.Background())
	fetchData := func(modelChan chan types.Modeler[types.RawStatement], errorChan chan error) {
		for _, mon := range monitorArray {
			if sliceOfMaps, cnt, err := monitor.AsSliceOfMaps[types.SimpleTransaction](&mon, filter); err != nil {
				errorChan <- err
				cancel()

			} else if cnt == 0 {
				errorChan <- fmt.Errorf("no appearances found for %s", mon.Address.Hex())
				continue

			} else {
				bar := logger.NewBar(logger.BarOptions{
					Prefix:  mon.Address.Hex(),
					Enabled: !testMode && !utils.IsTerminal(),
					Total:   int64(cnt),
				})

				for _, thisMap := range sliceOfMaps {
					thisMap := thisMap
					for app := range thisMap {
						thisMap[app] = new(types.SimpleTransaction)
					}

					iterFunc := func(app types.SimpleAppearance, value *types.SimpleTransaction) error {
						if tx, err := opts.Conn.GetTransactionByAppearance(&app, false); err != nil {
							return err
						} else {
							passes, _ := filter.ApplyTxFilters(tx)
							if passes {
								*value = *tx
							}
							if bar != nil {
								bar.Tick()
							}
							return nil
						}
					}

					// Set up and interate over the map calling iterFunc for each appearance
					iterCtx, iterCancel := context.WithCancel(context.Background())
					defer iterCancel()
					errChan := make(chan error)
					go utils.IterateOverMap(iterCtx, errChan, thisMap, iterFunc)
					if stepErr := <-errChan; stepErr != nil {
						errorChan <- stepErr
						iterCancel()
						return
					}

					txArray := make([]*types.SimpleTransaction, 0, len(thisMap))
					for _, tx := range thisMap {
						txArray = append(txArray, tx)
					}

					sort.Slice(txArray, func(i, j int) bool {
						if txArray[i].BlockNumber == txArray[j].BlockNumber {
							return txArray[i].TransactionIndex < txArray[j].TransactionIndex
						}
						return txArray[i].BlockNumber < txArray[j].BlockNumber
					})

					// Sort the items back into an ordered array by block number
					items := make([]*types.SimpleStatement, 0, len(thisMap))

					chain := opts.Globals.Chain
					testMode := opts.Globals.TestMode
					ledgers := ledger.NewLedger(
						opts.Conn,
						mon.Address,
						opts.FirstBlock,
						opts.LastBlock,
						opts.Globals.Ether,
						testMode,
						opts.NoZero,
						opts.Traces,
						&opts.Asset,
					)

					apps := make([]types.SimpleAppearance, 0, len(thisMap))
					for _, tx := range txArray {
						apps = append(apps, types.SimpleAppearance{
							BlockNumber:      uint32(tx.BlockNumber),
							TransactionIndex: uint32(tx.TransactionIndex),
						})
					}
					_ = ledgers.SetContexts(chain, apps, filter.GetOuterBounds())

					// we need them sorted for the following to work
					for _, tx := range txArray {
						ledgers.Tx = tx // we need this below
						if stmts := ledgers.GetStatementsFromTransaction(opts.Conn, filter, tx); len(stmts) > 0 {
							for _, statement := range stmts {
								statement := statement
								items = append(items, statement)
							}
						}
					}

					sort.Slice(items, func(i, j int) bool {
						if opts.Reversed {
							i, j = j, i
						}
						itemI := items[i]
						itemJ := items[j]
						if itemI.BlockNumber == itemJ.BlockNumber {
							if itemI.TransactionIndex == itemJ.TransactionIndex {
								return itemI.LogIndex < itemJ.LogIndex
							}
							return itemI.TransactionIndex < itemJ.TransactionIndex
						}
						return itemI.BlockNumber < itemJ.BlockNumber
					})

					for _, statement := range items {
						statement := statement
						modelChan <- statement
					}
				}

				bar.Finish(true /* newLine */)
			}
		}
	}

	extra := map[string]interface{}{
		"articulate": opts.Articulate,
		"testMode":   testMode,
		"export":     true,
	}

	if opts.Globals.Verbose || opts.Globals.Format == "json" {
		parts := names.Custom | names.Prefund | names.Regular
		namesMap, err := names.LoadNamesMap(chain, parts, nil)
		if err != nil {
			return err
		}
		extra["namesMap"] = namesMap
	}

	return output.StreamMany(ctx, fetchData, opts.Globals.OutputOptsWithExtra(extra))
}
