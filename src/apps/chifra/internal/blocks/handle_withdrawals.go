// Copyright 2021 The TrueBlocks Authors. All rights reserved.
// Use of this source code is governed by a license that can
// be found in the LICENSE file.

package blocksPkg

import (
	"context"
	"fmt"
	"sort"

	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/identifiers"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/logger"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/output"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/types"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/utils"
)

func (opts *BlocksOptions) HandleWithdrawals() error {
	chain := opts.Globals.Chain
	testMode := opts.Globals.TestMode
	nErrors := 0

	ctx, cancel := context.WithCancel(context.Background())
	fetchData := func(modelChan chan types.Modeler[types.RawWithdrawal], errorChan chan error) {
		if sliceOfMaps, cnt, err := identifiers.AsSliceOfMaps[types.SimpleBlock[string]](chain, opts.BlockIds); err != nil {
			errorChan <- err
			cancel()

		} else if cnt == 0 {
			errorChan <- fmt.Errorf("no blocks found for the query")
			cancel()

		} else {
			bar := logger.NewBar(logger.BarOptions{
				Enabled: !testMode && !utils.IsTerminal(),
				Total:   int64(cnt),
			})

			for _, thisMap := range sliceOfMaps {
				thisMap := thisMap
				for app := range thisMap {
					thisMap[app] = new(types.SimpleBlock[string])
				}

				items := make([]*types.SimpleWithdrawal, 0, len(thisMap))
				iterFunc := func(app types.SimpleAppearance, value *types.SimpleBlock[string]) error {
					bn := uint64(app.BlockNumber)
					if block, err := opts.Conn.GetBlockHeaderByNumber(bn); err != nil {
						delete(thisMap, app)
						return err
					} else {
						*value = block
						bar.Tick()
					}
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

				for _, item := range thisMap {
					for _, w := range item.Withdrawals {
						w := w
						w.BlockNumber = item.BlockNumber
						items = append(items, &w)
					}
				}

				sort.Slice(items, func(i, j int) bool {
					if items[i].BlockNumber == items[j].BlockNumber {
						return items[i].Index < items[j].Index
					}
					return items[i].BlockNumber < items[j].BlockNumber
				})

				for _, item := range items {
					item := item
					modelChan <- item
				}
			}
			bar.Finish(true /* newLine */)
		}
	}

	extra := map[string]interface{}{
		"ether": opts.Globals.Ether,
	}

	return output.StreamMany(ctx, fetchData, opts.Globals.OutputOptsWithExtra(extra))
}
