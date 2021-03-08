package mapper

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var descRegexp = regexp.MustCompile(`\\\[[a-z_, ]*\\\]`)

func parseEvents(log *zap.SugaredLogger, eventRes *eventpb.GetByHeightResponse, currency string, divider *big.Float, exp int) (eventMap, error) {
	evIndexMap := make(map[int64]struct{})

	events := make(map[int64][]structs.TransactionEvent)
	for _, e := range eventRes.Events {
		if !isEventUnique(evIndexMap, e.Index) {
			continue
		}

		if _, ok := events[e.ExtrinsicIndex]; !ok {
			events[e.ExtrinsicIndex] = make([]structs.TransactionEvent, 0)
		}

		event, err := getEvent(log, e, currency, exp, divider)
		if err != nil {
			return nil, err
		}

		kind := ""
		if event.Error != nil {
			kind = "error"
		}

		events[e.ExtrinsicIndex] = append(events[e.ExtrinsicIndex], structs.TransactionEvent{
			ID:   strconv.FormatInt(e.Index, 10),
			Kind: kind,
			Sub:  []structs.SubsetEvent{event},
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

type event struct {
	structs.SubsetEvent

	accountIDs         []string
	recipientAccountID string
	senderAccountID    string
	value              string
	amount             structs.TransactionAmount

	currency  string
	eventType []string
}

func getEvent(log *zap.SugaredLogger, evpb *eventpb.Event, currency string, exp int, divider *big.Float) (structs.SubsetEvent, error) {
	var e event

	if err := e.parseEventDescription(log, evpb); err != nil {
		return structs.SubsetEvent{}, err
	}

	if err := e.appendAmounts(exp, currency, divider); err != nil {
		return structs.SubsetEvent{}, err
	}

	e.appendAccounts()

	e.Module = evpb.Section

	e.Type = []string{evpb.Method}
	if e.eventType != nil {
		e.Type = append(e.Type, e.eventType...)
	}

	if evpb.Error != "" {
		e.Error = &structs.SubsetEventError{
			Message: evpb.Error,
		}
	}

	return e.SubsetEvent, nil
}

func (e *event) parseEventDescription(log *zap.SugaredLogger, ev *eventpb.Event) error {
	dataLen := len(ev.Data)
	attributes := make([]string, dataLen)

	values, err := getValues(ev.Description)
	if err != nil {
		return err
	}

	for i, v := range values {
		if i >= dataLen {
			return fmt.Errorf("Not enough data to parse all event values")
		}

		switch v {
		case "account", "approving", "authority_id", "multisig", "stash", "unvested":
			if accountID, err := getAccountID(ev.Data[i]); err == nil {
				e.accountIDs = append(e.accountIDs, accountID)
			}
		case "from":
			e.senderAccountID, err = getAccountID(ev.Data[i])
		case "to", "who":
			e.recipientAccountID, err = getAccountID(ev.Data[i])
		case "deposit", "free_balance", "value", "balance", "amount":
			if balance, err := getBalance(ev.Data[i]); err == nil {
				e.value = balance
			}
		case "error":
			e.eventType = []string{"error"}
		case "info", "tip_hash", "call_hash", "index":
			break
		default:
			return fmt.Errorf("Unknown value to parse event %q values: %v", v, values)
		}

		if err != nil {
			return fmt.Errorf("%d Error while parsing event %s", i, err.Error())
		}

		attributes[i] = fmt.Sprintf("%v", ev.Data[i])
	}

	if dataLen > 0 {
		e.Additional = make(map[string][]string)
		e.Additional["attributes"] = attributes
	}

	return nil
}

func getValues(description string) ([]string, error) {
	vls := string(descRegexp.Find([]byte(description)))
	if len(vls) < 5 {
		// Arguments are not required in description
		return nil, nil
	}
	vls = vls[2 : len(vls)-2]

	values := strings.Split(vls, ",")
	for i, v := range values {
		values[i] = strings.TrimSpace(v)
	}

	return values, nil
}

func getValue(data *eventpb.EventData, expected string) (string, error) {
	if data.Name != expected {
		return "", fmt.Errorf("unexpected data name %q expected %q value %q", data.Name, expected, data.Value)
	}

	return data.Value, nil
}

func getAccountID(data *eventpb.EventData) (string, error) {
	return getValue(data, "AccountId")
}

func getBalance(data *eventpb.EventData) (string, error) {
	return getValue(data, "Balance")
}

func getAmount(value string, exp int, currency string, divider *big.Float) (*structs.TransactionAmount, error) {
	if value == "" {
		return nil, nil
	}

	n := new(big.Int)
	n, ok := n.SetString(value, 10)
	if !ok {
		return nil, fmt.Errorf("Could not create big int from value %s", value)
	}

	amount, err := countCurrencyAmount(exp, value, divider)
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

func (e *event) appendAmounts(exp int, currency string, divider *big.Float) error {
	if e.value == "" {
		return nil
	}

	amount, err := getAmount(e.value, exp, currency, divider)
	if err != nil {
		return err
	}

	e.amount = *amount

	e.Amount = make(map[string]structs.TransactionAmount)
	e.Amount["0"] = *amount
	return nil
}

func (e *event) appendAccounts() {
	if e.accountIDs == nil && e.senderAccountID == "" && e.recipientAccountID == "" {
		return
	}

	e.Node = make(map[string][]structs.Account)

	accounts := make([]structs.Account, len(e.accountIDs))
	for i, accountID := range e.accountIDs {
		accounts[i] = structs.Account{
			ID: accountID,
		}
	}
	if e.accountIDs != nil {
		e.Node["versions"] = accounts
	}

	e.appendSender()
	e.appendRecipient()
}

func (e *event) appendSender() {
	if e.senderAccountID == "" {
		return
	}

	account := structs.Account{
		ID: e.senderAccountID,
	}

	eventTransfer := structs.EventTransfer{
		Account: account,
		Amounts: []structs.TransactionAmount{e.amount},
	}

	e.Sender = []structs.EventTransfer{
		eventTransfer,
	}

	e.Node["sender"] = []structs.Account{
		account,
	}
}

func (e *event) appendRecipient() {
	if e.recipientAccountID == "" {
		return
	}

	account := structs.Account{
		ID: e.recipientAccountID,
	}

	eventTransfer := structs.EventTransfer{
		Account: account,
		Amounts: []structs.TransactionAmount{e.amount},
	}

	e.Recipient = []structs.EventTransfer{
		eventTransfer,
	}

	e.Node["recipient"] = []structs.Account{
		account,
	}

	transfers := make(map[string][]structs.EventTransfer)
	transfers[e.recipientAccountID] = []structs.EventTransfer{eventTransfer}
	e.Transfers = transfers
}
