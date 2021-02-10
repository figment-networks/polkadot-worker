package mapper

import (
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/figment-networks/polkadot-worker/proxy"
	"github.com/pkg/errors"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
)

// TransactionMapper maps Block and Transaction response into database Transcation struct
func TransactionMapper(blockRes *blockpb.GetByHeightResponse, eventRes *eventpb.GetByHeightResponse, transactionRes *transactionpb.GetByHeightResponse, chainID, version string) ([]*structs.Transaction, error) {
	timer := metrics.NewTimer(proxy.TransactionConversionDuration)
	defer timer.ObserveDuration()

	if blockRes == nil || eventRes == nil || transactionRes == nil {
		return nil, nil
	}

	blockHash := blockRes.Block.BlockHash
	height := blockRes.Block.Header.Height
	transactionMap := make(map[string]struct{})

	var transactions []*structs.Transaction
	for _, t := range transactionRes.Transactions {
		if !isUnique(transactionMap, t.Hash) {
			continue
		}

		time, err := parseTime(t.Time)
		if err != nil {
			return nil, err
		}

		feeInt, err := strconv.Atoi(t.PartialFee)
		if err != nil {
			return nil, errors.Wrap(err, "Could not parse transaction partial fee")
		}

		fee := []structs.TransactionAmount{{
			Text:     t.PartialFee,
			Currency: "",
			Numeric:  big.NewInt(int64(feeInt)),
			Exp:      0,
		}}

		events := []structs.TransactionEvent{}
		for _, e := range eventRes.Events {
			value := ""
			if len(e.Data) > 0 {
				value = e.Data[0].Value
				for _, d := range e.Data {
					fmt.Println("name ", d.Name, " value ", d.Value)
				}
			}

			amount := structs.TransactionAmount{
				Text:     value,
				Currency: "",
				Numeric:  nil,
				Exp:      0,
			}

			if e.ExtrinsicIndex == t.ExtrinsicIndex {
				sender := structs.EventTransfer{
					Account: structs.Account{
						ID: "",
						Details: &structs.AccountDetails{
							Description: "",
							Contact:     "",
							Name:        "",
							Website:     "",
						},
					},
					Amounts: []structs.TransactionAmount{amount},
				}
				recipient := structs.EventTransfer{
					Account: structs.Account{
						ID: "",
						Details: &structs.AccountDetails{
							Description: "",
							Contact:     "",
							Name:        "",
							Website:     "",
						},
					},
					Amounts: []structs.TransactionAmount{amount},
				}

				// ??? what is the key ???
				transfers := make(map[string][]structs.EventTransfer, 0)
				transfers[""] = []structs.EventTransfer{sender}
				transfers[""] = []structs.EventTransfer{recipient}

				subs := []structs.SubsetEvent{{
					Type:       []string{},
					Action:     "",
					Module:     "",
					Sender:     []structs.EventTransfer{sender},
					Recipient:  []structs.EventTransfer{recipient},
					Node:       map[string][]structs.Account{},
					Nonce:      strconv.Itoa(int(t.Nonce)),
					Completion: &time,
					Amount:     map[string]structs.TransactionAmount{},
					Transfers:  transfers,
					Error:      &structs.SubsetEventError{},
					Additional: map[string][]string{},
					Sub:        []structs.SubsetEvent{},
				}}

				events = append(events, structs.TransactionEvent{
					ID:   strconv.Itoa(int(e.Index)),
					Kind: "",
					Sub:  subs,
				})
			}
		}

		fmt.Println("")

		transactions = append(transactions, &structs.Transaction{
			Hash:      t.Hash,
			BlockHash: blockHash,
			Height:    uint64(height),
			Epoch:     t.Time,
			ChainID:   chainID,
			Time:      time,
			Fee:       fee,
			GasUsed:   0,
			Memo:      "",
			Version:   version,
			Events:    events,
			Raw:       []byte{},
			RawLog:    []byte{},
			HasErrors: !t.IsSuccess,
		})
	}

	return transactions, nil
}

func isUnique(transactionMap map[string]struct{}, hash string) bool {
	if _, ok := transactionMap[hash]; !ok {
		transactionMap[hash] = struct{}{}
		return true
	}

	return false
}

func parseTime(timeStr string) (*time.Time, error) {
	timeInt, err := strconv.Atoi(timeStr)
	if err != nil {
		return nil, errors.Wrap(err, "Could not parse transaction time")
	}

	time := time.Unix(int64(timeInt), 0)
	return &time, nil
}
