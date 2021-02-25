package mapper

import (
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/figment-networks/polkadot-worker/proxy"
	"go.uber.org/zap"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/pkg/errors"
)

// TransactionsMapper maps Block and Transactions response into database Transcations struct
func TransactionsMapper(log *zap.SugaredLogger, blockRes *blockpb.GetByHeightResponse, eventRes *eventpb.GetByHeightResponse,
	transactionRes *transactionpb.GetByHeightResponse, exp int, div *big.Float, chainID, currency, version string) ([]*structs.Transaction, error) {
	var transactions []*structs.Transaction
	transactionMap := make(map[string]struct{})

	timer := metrics.NewTimer(proxy.TransactionConversionDuration)
	defer timer.ObserveDuration()

	if blockRes == nil || eventRes == nil || transactionRes == nil {
		return nil, nil
	}

	allEvents, err := parseEvents(log, eventRes, currency, div, exp)
	if err != nil {
		return nil, err
	}

	for _, t := range transactionRes.Transactions {
		if !isTransactionUnique(transactionMap, t.Hash) {
			continue
		}

		time, err := parseTime(t.Time)
		if err != nil {
			return nil, err
		}

		fee, err := getTransactionFee(exp, div, currency, t.PartialFee, t.Tip)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, &structs.Transaction{
			Hash:      t.Hash,
			BlockHash: blockRes.Block.BlockHash,
			Height:    uint64(blockRes.Block.Header.Height),
			ChainID:   chainID,
			Time:      *time,
			Fee:       fee,
			Version:   version,
			Events:    allEvents.getEventsByTrIndex(t.ExtrinsicIndex, strconv.FormatUint(uint64(t.Nonce), 10), t.Hash, time),
			HasErrors: !t.IsSuccess,
			Raw:       []byte(t.Raw),
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

func getTransactionFee(exp int, divider *big.Float, currency, partialFeeStr, tipStr string) ([]structs.TransactionAmount, error) {
	var ok bool
	var fee, tip *big.Int

	if fee, ok = new(big.Int).SetString(partialFeeStr, 10); !ok {
		return nil, fmt.Errorf("Could not parse transaction partial fee %q", partialFeeStr)
	}

	if tip, ok = new(big.Int).SetString(tipStr, 10); !ok {
		return nil, fmt.Errorf("Could not parse transaction tip %q", tipStr)
	}

	amount := new(big.Int).Add(fee, tip)

	textAmount, err := countCurrencyAmount(exp, amount.String(), divider)
	if err != nil {
		return nil, errors.New("Could not count currency amount")
	}

	return []structs.TransactionAmount{{
		Text:     fmt.Sprintf("%s%s", textAmount.Text('f', -1), currency),
		Currency: currency,
		Numeric:  amount,
		Exp:      int32(exp),
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

func countCurrencyAmount(exp int, value string, divider *big.Float) (*big.Float, error) {
	amount := new(big.Float)
	amount, ok := amount.SetString(value)
	if !ok {
		return nil, fmt.Errorf("Could not create big float from value %s", value)
	}

	return new(big.Float).Quo(amount, divider), nil
}
