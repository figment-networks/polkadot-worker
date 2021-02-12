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
	"go.uber.org/zap"

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
func TransactionMapper(log *zap.SugaredLogger, blockRes *blockpb.GetByHeightResponse, eventRes *eventpb.GetByHeightResponse,
	transactionRes *transactionpb.GetByHeightResponse, chainID, currency, version string) ([]*structs.Transaction, error) {
	timer := metrics.NewTimer(proxy.TransactionConversionDuration)
	defer timer.ObserveDuration()

	if blockRes == nil || eventRes == nil || transactionRes == nil {
		return nil, nil
	}

	blockHash := blockRes.Block.BlockHash
	height := blockRes.Block.Header.Height
	transactionMap := make(map[string]struct{})

	fmt.Println("blockHash: ", blockHash, "\nheight: ", height)

	allEvents, err := parseEvents(log, eventRes, currency)
	if err != nil {
		return nil, err
	}

	var transactions []*structs.Transaction
	for _, t := range transactionRes.Transactions {
		if !isTransactionUnique(transactionMap, t.Hash) {
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

		fmt.Println("FEE \n[]TRANSACTION AMOUNT\n(text,currency,numeric,exp)")

		fee := []structs.TransactionAmount{{
			Text:     t.PartialFee,
			Currency: currency,
			Numeric:  big.NewInt(int64(feeInt)),
			Exp:      0,
		}}

		fmt.Println("TRANSACTION\nHash:", t.Hash, "\nblockHash: ", blockHash, "\nheight: ", height, "\nuheight: ", uint64(height), "\nepoch: ", t.Time, "\nhas errors: ", !t.IsSuccess)

		fmt.Println("time, nonce:", time, t.Nonce)

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
			Events:    allEvents.getEventsByTrIndex(t.ExtrinsicIndex, strconv.Itoa(int(t.Nonce)), t.Hash, time),
			Raw:       []byte{},
			RawLog:    []byte{},
			HasErrors: !t.IsSuccess,
		})
	}

	return transactions, nil
}

func isTransactionUnique(transactionMap map[string]struct{}, hash string) bool {
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

type eventMap map[int64][]structs.TransactionEvent

func (e eventMap) getEventsByTrIndex(index int64, nonce, trHash string, time *time.Time) []structs.TransactionEvent {
	evts, ok := e[index]
	if !ok {
		return nil
	}

	events := make([]structs.TransactionEvent, len(evts))

	for i, evt := range evts {
		event := evt

		event.Sub = subWithNonceAndTime(&event.Sub, nonce, time)

		events[i] = event
	}

	return events
}

func subWithNonceAndTime(subs *[]structs.SubsetEvent, nonce string, time *time.Time) []structs.SubsetEvent {
	events := make([]structs.SubsetEvent, len(*subs))

	for i, sub := range *subs {
		event := sub

		event.Completion = time
		event.Nonce = nonce
		event.Sub = subWithNonceAndTime(&event.Sub, nonce, time)

		events[i] = event
	}

	return events
}

func parseEvents(log *zap.SugaredLogger, eventRes *eventpb.GetByHeightResponse, currency string) (eventMap, error) {
	evIndexMap := make(map[int64]struct{})
	trIndexMap := make(map[int64]struct{})

	events := make(map[int64][]structs.TransactionEvent)
	for _, e := range eventRes.Events {
		if !isEventUnique(evIndexMap, e.Index) {
			continue
		}

		if _, ok := trIndexMap[e.ExtrinsicIndex]; !ok {
			trIndexMap[e.ExtrinsicIndex] = struct{}{}
			events[e.ExtrinsicIndex] = make([]structs.TransactionEvent, 0)
		}

		fmt.Println("phase: ", e.Phase, " method: ", e.Method)

		var sub structs.SubsetEvent
		var eventError *structs.SubsetEventError
		var kind, module string
		var eventType []string
		subs := make([]structs.SubsetEvent, 0)
		node := make(map[string][]structs.Account)

		accounts := make([]structs.Account, 0)
		transferMap := make(map[string][]structs.EventTransfer)

		ev, err := getEventValues(log, e)
		if err != nil {
			return nil, err
		}

		appendDispatchInfo(ev.dispatchInfo)
		appendDispatchError(ev.dispatchError, eventError, &eventType, &kind, &module)

		appendAccount(ev.accountID, &accounts)
		amount := getAmount(ev.value, currency)
		appendSender(ev.accountID, &accounts, &amount, &sub.Sender)
		appendRecipient(ev.accountID, &accounts, &amount, &sub.Recipient, transferMap)

		node["versions"] = accounts
		sub.Node = node

		sub.Transfers = transferMap
		sub.Type = eventType

		if eventError != nil {
			sub.Error = eventError
		}

		// sub.Action = ?
		// sub.Module = ?
		// sub.Additional = ?

		subs = append(subs, sub)

		events[e.ExtrinsicIndex] = append(events[e.ExtrinsicIndex], structs.TransactionEvent{
			ID:   strconv.Itoa(int(e.Index)),
			Kind: kind,
			Sub:  subs,
		})

	}

	return events, nil
}

func isEventUnique(evIndexMap map[int64]struct{}, evIdx int64) bool {
	if _, ok := evIndexMap[evIdx]; !ok {
		evIndexMap[evIdx] = struct{}{}
		return true
	}
	return false
}

type eventValues struct {
	accountID          *string
	senderAccountID    *string
	recipientAccountID *string
	dispatchInfo       *dispatchInfo
	dispatchError      *dispatchError
	value              *string
}

func getEventValues(log *zap.SugaredLogger, event *eventpb.Event) (ev eventValues, err error) {
	dataLen := len(event.Data)

	values := getValues(event.Description)
	fmt.Printf("values: %q\n", values)
	for i, v := range values {
		if i >= dataLen {
			err = fmt.Errorf("Not enough data to parse all event values")
			return
		}

		switch v {
		case "account":
			ev.accountID = getAccountID(event.Data[i])
		case "error":
			ev.dispatchError, err = getDispatchError(event.Data[i])
			if err != nil {
				return eventValues{}, err
			}
		case "from":
			ev.senderAccountID = getAccountID(event.Data[i])
		case "info":
			ev.dispatchInfo, err = getDispatchInfo(event.Data[i])
			if err != nil {
				return eventValues{}, err
			}
		case "to", "who":
			ev.recipientAccountID = getAccountID(event.Data[i])
		case "deposit", "free_balance", "value":
			ev.value = getBalance(event.Data[i])
		default:
			log.Error("Unknown value to parse event", zap.String("event_value", v))
		}
	}

	return
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

func getValue(data *eventpb.EventData, expected string) *string {
	if data.Name != expected {
		fmt.Printf("unexpected data name %q expected %q value %q", data.Name, expected, data.Value)
	}

	return &data.Value
}

func getAccountID(data *eventpb.EventData) *string {
	return getValue(data, "AccountId")
}

func getBalance(data *eventpb.EventData) *string {
	return getValue(data, "Balance")
}

type dispatchInfo struct {
	weight  float64
	class   string
	paysFee string
}

func getDispatchInfo(data *eventpb.EventData) (*dispatchInfo, error) {
	var result map[string]interface{}

	value := getValue(data, "DispatchInfo")

	if err := json.Unmarshal([]byte(*value), &result); err != nil {
		return nil, errors.Wrap(err, "Could not unmarshall dispatch info")
	}

	weight, ok := result["weight"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not parse weight from %q", *value))
	}

	class, ok := result["class"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not parse class from %q", *value))
	}

	paysFee, ok := result["paysFee"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not parse paysFee from %q", *value))
	}

	fmt.Println(" weight class paysFee: ", weight, class, paysFee)

	return &dispatchInfo{
		weight:  weight.(float64),
		class:   class.(string),
		paysFee: paysFee.(string),
	}, nil
}

type dispatchError struct {
	module module
}

type module struct {
	index float64
	err   float64
}

func getDispatchError(data *eventpb.EventData) (*dispatchError, error) {
	var result map[string]interface{}

	value := getValue(data, "DispatchError")

	if err := json.Unmarshal([]byte(*value), &result); err != nil {
		return nil, errors.Wrap(err, "Could not unmarshall dispatch info")
	}

	errModule, ok := result["Module"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not parse Module from %q", *value))
	}

	index, ok := errModule.(map[string]interface{})["index"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not parse index from %q", *value))
	}

	eventError, ok := errModule.(map[string]interface{})["error"]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Could not parse error from %q", *value))
	}

	fmt.Println(" index error: ", index, eventError)

	return &dispatchError{
		module: module{
			index: index.(float64),
			err:   eventError.(float64),
		},
	}, nil
}

func appendAccount(accountID *string, accounts *[]structs.Account) {
	if accountID == nil {
		return
	}
	account := structs.Account{
		ID: *accountID,
	}

	*accounts = append(*accounts, account)
}

func appendDispatchInfo(dispatchInfo *dispatchInfo) {
	if dispatchInfo == nil {
		return
	}
	fmt.Printf("dispatchInfo: %3v\n\n%v\n", &dispatchInfo, &dispatchInfo)
}

func appendDispatchError(dispatchError *dispatchError, err *structs.SubsetEventError, eventType *[]string, kind, module *string) {
	if dispatchError == nil {
		return
	}

	fmt.Println("dispatch error index: ", dispatchError.module.index, " error: ", dispatchError.module.err, " module: ", dispatchError.module)

	*kind = "error"
	*eventType = []string{"error"}
	// ???
	*module = "exit code?"
	err = &structs.SubsetEventError{
		// Message: dispatchError.module.error,
		Message: "error message",
	}

	fmt.Printf("dispatchError: %3v\n", &dispatchError)
}

func getAmount(value *string, currency string) []structs.TransactionAmount {
	if value == nil {
		return nil
	}

	n := new(big.Int)
	n, ok := n.SetString(*value, 10)
	if !ok {
		fmt.Println("could not create big int from value ", value)
	}

	trAmount := structs.TransactionAmount{
		Text:     *value,
		Currency: currency,
		Numeric:  n,
		Exp:      0,
	}

	return []structs.TransactionAmount{trAmount}
}

func appendSender(accountID *string, accounts *[]structs.Account, amount *[]structs.TransactionAmount, sender *[]structs.EventTransfer) {
	if accountID == nil {
		return
	}

	account := structs.Account{
		ID: *accountID,
	}

	eventTransfer := structs.EventTransfer{
		Account: account,
		Amounts: *amount,
	}

	*sender = []structs.EventTransfer{
		eventTransfer,
	}

	*accounts = append(*accounts, account)
}

func appendRecipient(accountID *string, accounts *[]structs.Account, amount *[]structs.TransactionAmount, recipient *[]structs.EventTransfer, transferMap map[string][]structs.EventTransfer) {
	if accountID == nil {
		return
	}
	account := structs.Account{
		ID: *accountID,
	}

	eventTransfer := structs.EventTransfer{
		Account: account,
		Amounts: *amount,
	}

	*recipient = []structs.EventTransfer{
		eventTransfer,
	}

	*accounts = append(*accounts, account)
	transferMap[*accountID] = []structs.EventTransfer{eventTransfer}
}
