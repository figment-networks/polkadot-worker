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

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	exp          = 10
	maxPrecision = 12
)

// TransactionMapper maps Transaction into database struct
type TransactionMapper struct {
	expMultipliers []*big.Int
	log            *zap.SugaredLogger
}

// New creates a new TransactionMapper
func New(log *zap.SugaredLogger) *TransactionMapper {
	tm := TransactionMapper{
		log: log,
	}
	tm.expMultipliers = make([]*big.Int, maxPrecision+1)
	for i := 0; i < maxPrecision; i++ {
		tm.expMultipliers[i] = new(big.Int).Exp(big.NewInt(10), big.NewInt(maxPrecision), nil)
	}
	return &tm
}

// Parse maps Block and Transaction response into database Transcation struct
func (m *TransactionMapper) Parse(blockRes *blockpb.GetByHeightResponse, eventRes *eventpb.GetByHeightResponse,
	transactionRes *transactionpb.GetByHeightResponse, chainID, currency, version string) ([]*structs.Transaction, error) {
	timer := metrics.NewTimer(proxy.TransactionConversionDuration)
	defer timer.ObserveDuration()

	if blockRes == nil || eventRes == nil || transactionRes == nil {
		return nil, nil
	}

	blockHash := blockRes.Block.BlockHash
	height := blockRes.Block.Header.Height
	transactionMap := make(map[string]struct{})

	allEvents, err := m.parseEvents(eventRes, currency, height)
	if err != nil {
		return nil, err
	}

	var transactions []*structs.Transaction
	for _, t := range transactionRes.Transactions {
		if !m.isTransactionUnique(transactionMap, t.Hash) {
			continue
		}

		time, err := m.parseTime(t.Time)
		if err != nil {
			return nil, err
		}

		fee, err := m.getTransactionFee(currency, t.PartialFee, t.Tip)
		if err != nil {
			return nil, err
		}

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

func (m *TransactionMapper) isTransactionUnique(transactionMap map[string]struct{}, hash string) bool {
	if _, ok := transactionMap[hash]; !ok {
		transactionMap[hash] = struct{}{}
		return true
	}

	return false
}

func (m *TransactionMapper) parseTime(timeStr string) (*time.Time, error) {
	timeInt, err := strconv.Atoi(timeStr)
	if err != nil {
		return nil, errors.Wrap(err, "Could not parse transaction time")
	}

	time := time.Unix(int64(timeInt), 0)
	return &time, nil
}

func (m *TransactionMapper) getTransactionFee(currency, partialFeeStr, tipStr string) ([]structs.TransactionAmount, error) {
	var ok bool
	var fee, tip *big.Int

	if fee, ok = new(big.Int).SetString(partialFeeStr, 10); !ok {
		return nil, errors.New("Could not parse transaction partial fee")
	}

	if tip, ok = new(big.Int).SetString(tipStr, 10); !ok {
		return nil, errors.New("Could not parse transaction tip")
	}

	amount := new(big.Int).Add(fee, tip)

	textAmount, err := m.countCurrencyAmount(amount.String())
	if err != nil {
		return nil, errors.New("Could not count currency amount")
	}

	return []structs.TransactionAmount{{
		Text:     fmt.Sprintf("%s%s", textAmount.Text('f', -1), currency),
		Currency: currency,
		Numeric:  amount,
		Exp:      exp,
	}}, nil
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

func (m *TransactionMapper) parseEvents(eventRes *eventpb.GetByHeightResponse, currency string, height int64) (eventMap, error) {
	evIndexMap := make(map[int64]struct{})
	trIndexMap := make(map[int64]struct{})

	events := make(map[int64][]structs.TransactionEvent)
	for _, e := range eventRes.Events {
		if !m.isEventUnique(evIndexMap, e.Index) {
			continue
		}

		if _, ok := trIndexMap[e.ExtrinsicIndex]; !ok {
			trIndexMap[e.ExtrinsicIndex] = struct{}{}
			events[e.ExtrinsicIndex] = make([]structs.TransactionEvent, 0)
		}

		var sub structs.SubsetEvent
		var kind, module string
		var eventType []string
		subs := make([]structs.SubsetEvent, 0)
		accounts := make([]structs.Account, 0)

		ev, err := m.getEventValues(e, &sub)
		if err != nil {
			return nil, err
		}

		appendAccount(ev.accountID, &accounts)
		appendDispatchError(ev.dispatchError, &sub, &eventType, &kind, &module)

		amount, err := m.getAmount(height, currency, ev.value)
		if err != nil {
			return nil, err
		}

		appendSender(ev.senderAccountID, &accounts, amount, &sub)
		appendRecipient(ev.recipientAccountID, &accounts, amount, &sub)

		sub.Module = e.Section

		if len(accounts) > 0 {
			node := make(map[string][]structs.Account)
			node["versions"] = accounts
			sub.Node = node
		}

		if amount != nil {
			sub.Amount = make(map[string]structs.TransactionAmount)
			sub.Amount["0"] = *amount
		}

		if eventType != nil {
			sub.Type = eventType
		} else {
			sub.Type = []string{e.Method}
		}

		subs = append(subs, sub)

		events[e.ExtrinsicIndex] = append(events[e.ExtrinsicIndex], structs.TransactionEvent{
			ID:   strconv.Itoa(int(e.Index)),
			Kind: kind,
			Sub:  subs,
		})

	}

	return events, nil
}

func (m *TransactionMapper) isEventUnique(evIndexMap map[int64]struct{}, evIdx int64) bool {
	if _, ok := evIndexMap[evIdx]; !ok {
		evIndexMap[evIdx] = struct{}{}
		return true
	}
	return false
}

type eventValues struct {
	accountID          *string
	recipientAccountID *string
	senderAccountID    *string
	value              *string
	dispatchError      *dispatchError
}

func (m *TransactionMapper) getEventValues(event *eventpb.Event, sub *structs.SubsetEvent) (ev eventValues, err error) {
	dataLen := len(event.Data)
	attributes := make([]string, dataLen)

	values, err := m.getValues(event.Description)
	if err != nil {
		return eventValues{}, err
	}

	for i, v := range values {
		if i >= dataLen {
			err = fmt.Errorf("Not enough data to parse all event values")
			return
		}

		switch v {
		case "account":
			ev.accountID, err = getAccountID(event.Data[i])
		case "error":
			ev.dispatchError, err = getDispatchError(event.Data[i])
		case "from":
			ev.senderAccountID, err = getAccountID(event.Data[i])
		case "to", "who":
			ev.recipientAccountID, err = getAccountID(event.Data[i])
		case "deposit", "free_balance", "value":
			ev.value, err = getBalance(event.Data[i])
		default:
			m.log.Error("Unknown value to parse event", zap.String("event_value", v))
		}

		if err != nil {
			return eventValues{}, err
		}

		attributes[i] = fmt.Sprintf("%v", event.Data[i])
	}

	if dataLen > 0 {
		sub.Additional = make(map[string][]string)
		sub.Additional["attributes"] = attributes
	}

	return
}

func (m *TransactionMapper) getValues(description string) ([]string, error) {
	vls := string(regexp.MustCompile(`\\\[.*\\\]`).Find([]byte(description)))
	if len(vls) < 5 {
		return nil, fmt.Errorf("Could not get values from description %q", description)
	}
	vls = vls[2 : len(vls)-2]

	values := strings.Split(vls, ",")
	for i, v := range values {
		values[i] = strings.TrimSpace(v)
	}

	return values, nil
}

func getValue(data *eventpb.EventData, expected string) (*string, error) {
	if data.Name != expected {
		return nil, fmt.Errorf("unexpected data name %q expected %q value %q", data.Name, expected, data.Value)
	}

	return &data.Value, nil
}

func getAccountID(data *eventpb.EventData) (*string, error) {
	return getValue(data, "AccountId")
}

func getBalance(data *eventpb.EventData) (*string, error) {
	return getValue(data, "Balance")
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

	value, err := getValue(data, "DispatchError")
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(*value), &result); err != nil {
		return nil, errors.Wrap(err, "Could not unmarshal dispatch info")
	}

	errModule, ok := result["Module"]
	if !ok {
		return nil, fmt.Errorf("Could not parse Module from %q", *value)
	}

	index, ok := errModule.(map[string]interface{})["index"]
	if !ok {
		return nil, fmt.Errorf("Could not parse index from %q", *value)
	}

	eventError, ok := errModule.(map[string]interface{})["error"]
	if !ok {
		return nil, fmt.Errorf("Could not parse error from %q", *value)
	}

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

func appendDispatchError(dispatchError *dispatchError, sub *structs.SubsetEvent, eventType *[]string, kind, module *string) {
	if dispatchError == nil {
		return
	}

	*kind = "error"
	*eventType = []string{"error"}
	*module = strconv.Itoa(int(dispatchError.module.index))

	sub.Error = &structs.SubsetEventError{
		Message: getErrorMsg(dispatchError.module.index, dispatchError.module.err),
	}
}

func (m *TransactionMapper) getAmount(height int64, currency string, value *string) (*structs.TransactionAmount, error) {
	if value == nil {
		return nil, nil
	}

	n := new(big.Int)
	n, ok := n.SetString(*value, 10)
	if !ok {
		return nil, fmt.Errorf("Could not create big int from value %s", *value)
	}

	amount, err := m.countCurrencyAmount(*value)
	if err != nil {
		return nil, errors.New("Could not count currency amount")
	}

	return &structs.TransactionAmount{
		Text:     fmt.Sprintf("%s%s", amount.Text('f', -1), currency),
		Currency: currency,
		Numeric:  n,
		Exp:      int32(exp),
	}, nil
}

func (m *TransactionMapper) countCurrencyAmount(value string) (*big.Float, error) {
	div := m.expMultipliers[exp]

	amount := new(big.Float)
	amount, ok := amount.SetString(value)
	if !ok {
		return nil, fmt.Errorf("Could not create big float from value %s", value)
	}

	return new(big.Float).Quo(amount, new(big.Float).SetFloat64(float64(div.Int64()))), nil
}

func appendSender(accountID *string, accounts *[]structs.Account, amount *structs.TransactionAmount, sub *structs.SubsetEvent) {
	if accountID == nil {
		return
	}

	account := structs.Account{
		ID: *accountID,
	}

	eventTransfer := structs.EventTransfer{
		Account: account,
		Amounts: []structs.TransactionAmount{*amount},
	}

	sub.Sender = []structs.EventTransfer{
		eventTransfer,
	}

	*accounts = append(*accounts, account)
}

func appendRecipient(accountID *string, accounts *[]structs.Account, amount *structs.TransactionAmount, sub *structs.SubsetEvent) {
	if accountID == nil {
		return
	}

	account := structs.Account{
		ID: *accountID,
	}

	eventTransfer := structs.EventTransfer{
		Account: account,
		Amounts: []structs.TransactionAmount{*amount},
	}

	sub.Recipient = []structs.EventTransfer{
		eventTransfer,
	}

	*accounts = append(*accounts, account)

	transfers := make(map[string][]structs.EventTransfer)
	transfers[*accountID] = []structs.EventTransfer{eventTransfer}
	sub.Transfers = transfers
}
