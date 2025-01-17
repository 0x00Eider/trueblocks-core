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
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/logger"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/monitor"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/names"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/output"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/types"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/utils"
)

func (opts *ExportOptions) HandleWithdrawals(monitorArray []monitor.Monitor) error {
	chain := opts.Globals.Chain
	testMode := opts.Globals.TestMode
	nErrors := 0
	first := utils.Max(base.KnownBlock(chain, "shanghai"), opts.FirstBlock)
	filter := filter.NewFilter(
		opts.Reversed,
		false,
		[]string{},
		base.BlockRange{First: first, Last: opts.LastBlock},
		base.RecordRange{First: first, Last: opts.GetMax()},
	)

	ctx, cancel := context.WithCancel(context.Background())
	fetchData := func(modelChan chan types.Modeler[types.RawWithdrawal], errorChan chan error) {
		for _, mon := range monitorArray {
			if sliceOfMaps, cnt, err := monitor.AsSliceOfMaps[types.SimpleBlock[string]](&mon, filter); err != nil {
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
						thisMap[app] = new(types.SimpleBlock[string])
					}

					iterFunc := func(app types.SimpleAppearance, value *types.SimpleBlock[string]) error {
						var block types.SimpleBlock[string]
						if block, err = opts.Conn.GetBlockHeaderByNumber(uint64(app.BlockNumber)); err != nil {
							return err
						}

						withdrawals := make([]types.SimpleWithdrawal, 0, 16)
						for _, w := range block.Withdrawals {
							if w.Address == mon.Address {
								withdrawals = append(withdrawals, w)
							}
						}
						if len(withdrawals) > 0 {
							block.Withdrawals = withdrawals
							*value = block
						}

						bar.Tick()
						return nil
					}

					iterErrorChan := make(chan error)
					iterCtx, iterCancel := context.WithCancel(context.Background())
					defer iterCancel()
					go utils.IterateOverMap(iterCtx, iterErrorChan, thisMap, iterFunc)
					for err := range iterErrorChan {
						if !testMode || nErrors == 0 {
							errorChan <- err
							nErrors++
						}
					}

					// Sort the items back into an ordered array by block number
					items := make([]*types.SimpleWithdrawal, 0, len(thisMap))
					for _, block := range thisMap {
						for _, with := range block.Withdrawals {
							items = append(items, &with)
						}
					}
					sort.Slice(items, func(i, j int) bool {
						if opts.Reversed {
							i, j = j, i
						}
						return items[i].BlockNumber < items[j].BlockNumber
					})

					for _, item := range items {
						modelChan <- item
					}
				}
				bar.Finish(true /* newLine */)
			}
		}
	}

	extra := map[string]interface{}{
		"testMode": testMode,
		"export":   true,
	}

	if opts.Globals.Verbose || opts.Globals.Format == "json" {
		parts := names.Custom | names.Prefund | names.Regular
		if namesMap, err := names.LoadNamesMap(chain, parts, nil); err != nil {
			return err
		} else {
			extra["namesMap"] = namesMap
		}
	}

	return output.StreamMany(ctx, fetchData, opts.Globals.OutputOptsWithExtra(extra))
}
