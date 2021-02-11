package mapper

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/figment-networks/polkadot-worker/proxy"
	"github.com/pkg/errors"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
)

type eventType string

var (
	accountIDEvent     eventType = "AccountId"
	balanceEvent       eventType = "Balance"
	dispatchInfoEvent  eventType = "DispatchInfo"
	dispatchErrorEvent eventType = "DispatchError"
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

	fmt.Println("blockHash: ", blockHash, "\nheight: ", height)

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

		fmt.Println("partial fee: ", t.PartialFee)

		fmt.Println("FEE ???\n[]TRANSACTION AMOUNT\n(text,currency,numeric,exp)")

		fee := []structs.TransactionAmount{{
			Text:     t.PartialFee,
			Currency: "",
			Numeric:  big.NewInt(int64(feeInt)),
			Exp:      0,
		}}

		parseEvents(eventRes, strconv.Itoa(int(t.Nonce)), time)

		fmt.Println("TRANSACTION\nHash:", t.Hash, "\nblockHash: ", blockHash, "\nheight: ", height, "\nepoch: ", t.Time, "\nhas errors: ", !t.IsSuccess)

		fmt.Println("(gasUsed,memo)")

		transactions = append(transactions, &structs.Transaction{
			Hash:      t.Hash,
			BlockHash: blockHash,
			Height:    uint64(height),
			Epoch:     t.Time,
			ChainID:   chainID,
			Time:      *time,
			Fee:       fee,
			GasUsed:   0,
			Memo:      "",
			Version:   version,
			Events:    nil,
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

	fmt.Println("not unique, skipping ", hash)
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

// type trCount struct {
// 	transaction []structs.TransactionEvent
// 	count
// }

func parseEvents(eventRes *eventpb.GetByHeightResponse, nonce string, time *time.Time) map[int64][]*structs.TransactionEvent {
	trIndexMap := make(map[int64]struct{})

	// key == trIndex
	events := make(map[int64][]*structs.TransactionEvent)
	for _, e := range eventRes.Events {
		if _, ok := trIndexMap[e.ExtrinsicIndex]; !ok {
			trIndexMap[e.ExtrinsicIndex] = struct{}{}
			events[e.ExtrinsicIndex] = make([]*structs.TransactionEvent, 0)
		}

		subs := make([]structs.SubsetEvent, 0)
		sub := structs.SubsetEvent{}

		// ?? key from what?
		node := make(map[string][]structs.Account)
		accounts := make([]structs.Account, 0)
		// ?? key from what??
		amounts := make(map[string]structs.TransactionAmount)
		// ?? key from what??
		transferMap := make(map[string][]structs.EventTransfer)
		transfers := make([]structs.EventTransfer, 0)

		var dispatchInfo *dispatchInfo
		var dispatchError *dispatchError
		accountID, senderAccountID, recipientID, value := "", "", "", ""

		dataLen := len(e.Data)

		values := getValues(e.Description)
		fmt.Printf("values: %q\n", values)
		for i, v := range values {
			if i >= dataLen {
				fmt.Println("fatal, should never happen")
				return nil
			}

			switch v {
			case "account", "who":
				accountID = getAccountID(e.Data[i])
				break
			case "error":
				dispatchError = getDispatchError(e.Data[i])
				break
			case "from":
				senderAccountID = getAccountID(e.Data[i])
				break
			case "info":
				dispatchInfo = getDispatchInfo(e.Data[i])
				break
			case "to":
				recipientID = getAccountID(e.Data[i])
				break
			case "deposit", "free_balance", "value":
				value = getBalance(e.Data[i])
				break
			default:
				fmt.Println("don't know what to do with ", v)
			}
		}

		if accountID != "" {
			account := structs.Account{
				ID: accountID,
			}

			accounts = append(accounts, account)
		}

		if dispatchInfo != nil {
			fmt.Printf("dispatchInfo: %3v\n", &dispatchInfo)
		}

		if dispatchError != nil {
			fmt.Printf("dispatchError: %3v\n", &dispatchError)
		}

		var senderAndReciverAmount []structs.TransactionAmount
		if value != "" {
			n := new(big.Int)
			n, ok := n.SetString(value, 10)
			if !ok {
				fmt.Println("could not create big int from value ", value)
			}

			amount := structs.TransactionAmount{
				Text: value,
				// ??
				Currency: "",
				Numeric:  n,
				// ??
				Exp: 0,
			}

			amounts[""] = amount

			senderAndReciverAmount = []structs.TransactionAmount{amount}
		} else {
			senderAndReciverAmount = nil
		}

		if senderAccountID != "" {
			account := structs.Account{
				ID: senderAccountID,
			}

			eventTransfer := structs.EventTransfer{
				Account: account,
				Amounts: senderAndReciverAmount,
			}

			sub.Sender = []structs.EventTransfer{
				eventTransfer,
			}

			accounts = append(accounts, account)
			transfers = append(transfers, eventTransfer)
		}

		if recipientID != "" {
			account := structs.Account{
				ID: recipientID,
			}

			eventTransfer := structs.EventTransfer{
				Account: account,
				Amounts: senderAndReciverAmount,
			}

			sub.Recipient = []structs.EventTransfer{
				eventTransfer,
			}

			accounts = append(accounts, account)
			transfers = append(transfers, eventTransfer)
		}

		sub.Completion = time
		node[""] = accounts
		sub.Node = node
		sub.Nonce = nonce
		transferMap[""] = transfers
		sub.Transfers = transferMap

		// sub.Type = ?
		// sub.Action = ?
		// sub.Module = ?
		// sub.Additional = ?

		subs = append(subs, sub)

		events[e.ExtrinsicIndex] = append(events[e.ExtrinsicIndex], &structs.TransactionEvent{
			ID: strconv.Itoa(int(e.Index)),
			// ??
			Kind: "",
			Sub:  subs,
		})

	}

	// events := make([]structs.TransactionEvent, 0)
	// for _, e := range eventRes.Events {
	// 	fmt.Println("\n\nIndex: ", e.Index, "\nDescription: ", e.Description, "\nmethod: ", e.Method, "\nphase: ", e.Phase, "\nsection: ", e.Section)
	// 	value := ""
	// 	for _, d := range e.Data {
	// 		value += d.Value
	// 		fmt.Println("name ", d.Name, " value ", d.Value)
	// 	}

	// 	fmt.Println("\nCurrency = ? Numeric = ? Exp = ?")

	// 	amount := structs.TransactionAmount{
	// 		Text:     value,
	// 		Currency: "",
	// 		Numeric:  nil,
	// 		Exp:      0,
	// 	}

	// 	if e.ExtrinsicIndex == trIndex {
	// 		sender := structs.EventTransfer{
	// 			Account: structs.Account{
	// 				ID: "",
	// 				Details: &structs.AccountDetails{
	// 					Description: "",
	// 					Contact:     "",
	// 					Name:        "",
	// 					Website:     "",
	// 				},
	// 			},
	// 			Amounts: []structs.TransactionAmount{amount},
	// 		}
	// 		recipient := structs.EventTransfer{
	// 			Account: structs.Account{
	// 				ID: "",
	// 				Details: &structs.AccountDetails{
	// 					Description: "",
	// 					Contact:     "",
	// 					Name:        "",
	// 					Website:     "",
	// 				},
	// 			},
	// 			Amounts: []structs.TransactionAmount{amount},
	// 		}

	// 		fmt.Println("EVENT TRANSFER\n(account,amounts)(in:sender,recipient)")

	// 		// ??? what is the key ???
	// 		transfers := make(map[string][]structs.EventTransfer, 0)
	// 		transfers[""] = []structs.EventTransfer{sender}
	// 		transfers[""] = []structs.EventTransfer{recipient}

	// 		fmt.Println("SUBSET EVENT\nnonce: ", nonce, "\n(type,action,module,sender,recipient,node(all accounts),completion,amount,transfers,error,additional,sub)")

	// 		subs := []structs.SubsetEvent{{
	// 			Type:       []string{},
	// 			Action:     "",
	// 			Module:     "",
	// 			Sender:     []structs.EventTransfer{sender},
	// 			Recipient:  []structs.EventTransfer{recipient},
	// 			Node:       map[string][]structs.Account{},
	// 			Nonce:      nonce,
	// 			Completion: time,
	// 			Amount:     map[string]structs.TransactionAmount{},
	// 			Transfers:  transfers,
	// 			Error:      &structs.SubsetEventError{},

	// 			// What should be here?
	// 			Additional: map[string][]string{},
	// 			Sub:        []structs.SubsetEvent{},
	// 		}}

	// 		fmt.Println("TRANSACTION EVENT\nkind = ? subs = ? ")

	// 		events = append(events, structs.TransactionEvent{
	// 			ID:   strconv.Itoa(int(e.Index)),
	// 			Kind: "",
	// 			Sub:  subs,
	// 		})
	// 	}
	// }
	return events
}

func getValues(description string) []string {
	vls := string(regexp.MustCompile(`\\\[.*\\\]`).Find([]byte(description)))
	vls = vls[2 : len(vls)-2]

	values := strings.Split(vls, ",")
	for i, v := range values {
		values[i] = strings.TrimSpace(v)
	}

	return values
}

func getValue(data *eventpb.EventData, expected string) string {
	if data.Name != expected {
		fmt.Printf("unexpected data name %q expected %q value %q", data.Name, expected, data.Value)
	}

	return data.Value
}

func getAccountID(data *eventpb.EventData) string {
	return getValue(data, "AccountId")
}

func getBalance(data *eventpb.EventData) string {
	return getValue(data, "Balance")
}

type dispatchInfo struct {
	weight         int64
	class, paysFee string
}

func getDispatchInfo(data *eventpb.EventData) (dispatchInfo *dispatchInfo) {
	value := getValue(data, "DispatchInfo")
	json.Unmarshal([]byte(value), dispatchInfo)
	return
}

type dispatchError struct {
	Module module
}

type module struct {
	index int64
	error int64
}

func getDispatchError(data *eventpb.EventData) (dispatchError *dispatchError) {
	value := getValue(data, "DispatchError")
	json.Unmarshal([]byte(value), dispatchError)
	return
}
