// Copyright 2021 The TrueBlocks Authors. All rights reserved.
// Use of this source code is governed by a license that can
// be found in the LICENSE file.
/*
 * Parts of this file were generated with makeClass --run. Edit only those parts of
 * the code inside of 'EXISTING_CODE' tags.
 */

package blocksPkg

// EXISTING_CODE
import (
	"net/http"

	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/internal/globals"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/logger"
	outputHelpers "github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/output/helpers"
	"github.com/spf13/cobra"
)

// EXISTING_CODE

// RunBlocks handles the blocks command for the command line. Returns error only as per cobra.
func RunBlocks(cmd *cobra.Command, args []string) error {
	opts := blocksFinishParse(args)
	outputHelpers.EnableCommand("blocks", true)
	// EXISTING_CODE
	// EXISTING_CODE
	outputHelpers.SetWriterForCommand("blocks", &opts.Globals)
	return opts.BlocksInternal()
}

// ServeBlocks handles the blocks command for the API. Returns an error.
func ServeBlocks(w http.ResponseWriter, r *http.Request) error {
	opts := blocksFinishParseApi(w, r)
	outputHelpers.EnableCommand("blocks", true)
	// EXISTING_CODE
	// EXISTING_CODE
	outputHelpers.InitJsonWriterApi("blocks", w, &opts.Globals)
	err := opts.BlocksInternal()
	outputHelpers.CloseJsonWriterIfNeededApi("blocks", err, &opts.Globals)
	return err
}

// BlocksInternal handles the internal workings of the blocks command.  Returns an error.
func (opts *BlocksOptions) BlocksInternal() error {
	var err error
	if err = opts.validateBlocks(); err != nil {
		return err
	}

	timer := logger.NewTimer()
	msg := "chifra blocks"
	// EXISTING_CODE
	if opts.Globals.Decache {
		err = opts.HandleDecache()

	} else if opts.Count {
		err = opts.HandleCounts()

	} else if opts.Logs {
		err = opts.HandleLogs()

	} else if opts.Withdrawals {
		err = opts.HandleWithdrawals()

	} else if opts.Traces {
		err = opts.HandleTraces()

	} else if opts.Uncles {
		err = opts.HandleUncles()

	} else if opts.List > 0 {
		err = opts.HandleList()

	} else if opts.Uniq {
		err = opts.HandleUniq()

	} else if opts.Hashes {
		err = opts.HandleHashes()

	} else {
		err = opts.HandleShow()
	}
	// EXISTING_CODE
	timer.Report(msg)

	return err
}

// GetBlocksOptions returns the options for this tool so other tools may use it.
func GetBlocksOptions(args []string, g *globals.GlobalOptions) *BlocksOptions {
	ret := blocksFinishParse(args)
	if g != nil {
		ret.Globals = *g
	}
	return ret
}

// EXISTING_CODE
// EXISTING_CODE
