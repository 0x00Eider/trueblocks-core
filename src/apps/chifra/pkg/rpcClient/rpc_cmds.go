// Copyright 2021 The TrueBlocks Authors. All rights reserved.
// Use of this source code is governed by a license that can
// be found in the LICENSE file.

package rpcClient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
)

// Traces carries values returned the `trace_block` RPC command
type Traces struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  []struct {
		Action struct {
			CallType      string `json:"callType"` // call
			From          string `json:"from"`
			Gas           string `json:"gas"`
			Input         string `json:"input"`
			To            string `json:"to"`
			Value         string `json:"value"`
			Author        string `json:"author"` // reward
			RewardType    string `json:"rewardType"`
			Address       string `json:"address"` // suicide
			Balance       string `json:"balance"`
			RefundAddress string `json:"refundAddress"`
			Init          string `json:"init"` // create
		} `json:"action,omitempty"`
		BlockHash   string `json:"blockHash"`
		BlockNumber int    `json:"blockNumber"`
		Error       string `json:"error"`
		Result      struct {
			GasUsed string `json:"gasUsed"` // call
			Output  string `json:"output"`
			Address string `json:"address"` // create
		} `json:"result"`
		Subtraces           int           `json:"subtraces"`
		TraceAddress        []interface{} `json:"traceAddress"`
		TransactionHash     string        `json:"transactionHash"`
		TransactionPosition int           `json:"transactionPosition"`
		Type                string        `json:"type"`
	} `json:"result"`
	ID int `json:"id"`
}

// Logs carries values returned by the eth_getLogs RPC command
type Logs struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  []struct {
		Address             string   `json:"address"`
		BlockHash           string   `json:"blockHash"`
		BlockNumber         string   `json:"blockNumber"`
		Data                string   `json:"data"`
		LogIndex            string   `json:"logIndex"`
		Removed             bool     `json:"removed"`
		Topics              []string `json:"topics"`
		TransactionHash     string   `json:"transactionHash"`
		TransactionIndex    string   `json:"transactionIndex"`
		TransactionLogIndex string   `json:"transactionLogIndex"`
		Type                string   `json:"type"`
	} `json:"result"`
	ID int `json:"id"`
}

// Transaction carries values returned by the eth_getTransacction RPC calls
type Transaction struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Hash             string
		Nonce            string
		BlockHash        string
		BlockNumber      string
		TransactionIndex string
		From             string
		To               string
		Value            string
		GasPrice         string
		Gas              string
		Input            string
		V                string
		StandardV        string
		R                string
		Raw              string
		Publickey        string
		Chainid          string
	} `json:"result"`
	ID int `json:"id"`
}

// LogFilter is used the eth_getLogs RPC call to identify the block range to query
type LogFilter struct {
	Fromblock string `json:"fromBlock"`
	Toblock   string `json:"toBlock"`
}

var rpcCounter uint32

// FromRpc Returns all traces for a given block.
func FromRpc(rpcProvider string, payload *RPCPayload, ret interface{}) error {
	type rpcPayload struct {
		Jsonrpc   string `json:"jsonrpc"`
		Method    string `json:"method"`
		RPCParams `json:"params"`
		ID        int `json:"id"`
	}
	payloadToSend := rpcPayload{
		Jsonrpc:   "2.0",
		Method:    payload.Method,
		RPCParams: payload.RPCParams,
		ID:        int(atomic.AddUint32(&rpcCounter, 1)),
	}
	plBytes, err := json.Marshal(payloadToSend)
	if err != nil {
		return err
	}

	body := bytes.NewReader(plBytes)
	req, err := http.NewRequest("POST", rpcProvider, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	theBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(theBytes, ret)
}

// RPCParams are used during calls to the RPC.
type RPCParams []interface{}

// RPCPayload is used during to make calls to the RPC.
type RPCPayload struct {
	Method    string `json:"method"`
	RPCParams `json:"params"`
}
