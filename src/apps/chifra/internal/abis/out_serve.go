package abisPkg

import (
	"net/http"
	"strings"

	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/cmd/globals"
)

func ServeAbis(w http.ResponseWriter, r *http.Request) {
	opts := FromRequest(w, r)

	err := opts.ValidateAbis()
	if err != nil {
		opts.Globals.RespondWithError(w, http.StatusInternalServerError, err)
		return
	}

	if len(opts.Find) > 0 {
		err = opts.FindInternal()
		if err != nil {
			opts.Globals.RespondWithError(w, http.StatusInternalServerError, err)
			return
		}
		return
	}

	globals.PassItOn("grabABI", &opts.Globals, opts.String(), strings.Join(opts.Addrs, " "))
}
