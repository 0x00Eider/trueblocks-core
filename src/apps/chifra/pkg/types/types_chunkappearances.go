// Copyright 2021 The TrueBlocks Authors. All rights reserved.
// Use of this source code is governed by a license that can
// be found in the LICENSE file.
/*
 * Parts of this file were generated with makeClass --run. Edit only those parts of
 * the code inside of 'EXISTING_CODE' tags.
 */

package types

// EXISTING_CODE
import (
	"io"
)

// EXISTING_CODE

type RawhunkAppearances struct {
	BlockNumber      string `json:"blockNumber"`
	TransactionIndex string `json:"transactionIndex"`
	// EXISTING_CODE
	// EXISTING_CODE
}

type SimplehunkAppearances struct {
	BlockNumber      uint64              `json:"blockNumber"`
	TransactionIndex uint64              `json:"transactionIndex"`
	raw              *RawhunkAppearances `json:"-"`
	// EXISTING_CODE
	// EXISTING_CODE
}

func (s *SimplehunkAppearances) Raw() *RawhunkAppearances {
	return s.raw
}

func (s *SimplehunkAppearances) SetRaw(raw *RawhunkAppearances) {
	s.raw = raw
}

func (s *SimplehunkAppearances) Model(showHidden bool, format string, extraOptions map[string]any) Model {
	var model = map[string]interface{}{}
	var order = []string{}

	// EXISTING_CODE
	// EXISTING_CODE

	return Model{
		Data:  model,
		Order: order,
	}
}

func (s *SimplehunkAppearances) WriteTo(w io.Writer) (n int64, err error) {
	// EXISTING_CODE
	// EXISTING_CODE
	return 0, nil
}

func (s *SimplehunkAppearances) ReadFrom(r io.Reader) (n int64, err error) {
	// EXISTING_CODE
	// EXISTING_CODE
	return 0, nil
}

// EXISTING_CODE
// EXISTING_CODE
