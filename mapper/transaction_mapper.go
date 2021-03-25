package mapper

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const chain_version string = "0.0.1"

func initExpDivider(precision int64) *big.Float {
	div := new(big.Int).Exp(big.NewInt(10), big.NewInt(precision), nil)
	return new(big.Float).SetFloat64(float64(div.Int64()))
}

type TransactionMapper struct {
	exp      int
	div      *big.Float
	chainID  string
	currency string
}

// NewTransactionMapper creates a new Transaction mapper
func NewTransactionMapper(exp int, chainID, currency string) *TransactionMapper {
	transactionConversionDuration = conversionDuration.WithLabels("transaction")
	return &TransactionMapper{
		exp:      exp,
		div:      initExpDivider(int64(exp)),
		chainID:  chainID,
		currency: currency,
	}
}

// TransactionsMapper maps Block and Transactions response into database Transcations struct
func (m *TransactionMapper) TransactionsMapper(log *zap.Logger, blockRes *blockpb.GetByHeightResponse, metaRes *chainpb.GetMetaByHeightResponse) ([]*structs.Transaction, error) {
	var transactions []*structs.Transaction
	transactionMap := make(map[string]struct{})

	timer := metrics.NewTimer(transactionConversionDuration)
	defer timer.ObserveDuration()

	if blockRes == nil || metaRes == nil {
		return nil, errors.New("Block or Meta response is empty")
	}

	for _, t := range blockRes.Block.Extrinsics {
		if !isTransactionUnique(transactionMap, t.Hash) {
			continue
		}

		time, err := parseTime(t.Time)
		if err != nil {
			return nil, err
		}

		events, err := parseEvents(log, t.GetEvents(), m.currency, m.div, m.exp, strconv.FormatUint(uint64(t.Nonce), 10), time)
		if err != nil {
			return nil, err
		}

		fee, err := m.getTransactionFee(t.PartialFee, t.Tip)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, &structs.Transaction{
			Hash:      t.Hash,
			BlockHash: blockRes.Block.BlockHash,
			Height:    uint64(blockRes.Block.Header.Height),
			Epoch:     strconv.FormatInt(metaRes.Era, 10),
			ChainID:   m.chainID,
			Time:      *time,
			Fee:       fee,
			Version:   chain_version,
			Events:    events,
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
	var err error
	var sec, nsec int

	if strings.Contains(timeStr, ".") {
		if slice := strings.Split(timeStr, "."); len(slice) > 1 {
			if nsec, err = parseInt(slice[1]); err != nil {
				return nil, err
			}
			timeStr = slice[0]
		}
	}

	if sec, err = parseInt(timeStr); err != nil {
		return nil, err
	}

	time := time.Unix(int64(sec), int64(nsec))
	return &time, nil
}

func parseInt(str string) (int, error) {
	result, err := strconv.Atoi(str)
	if err != nil {
		return 0, errors.Wrap(err, "Could not parse transaction time")
	}
	return result, nil
}

func (m *TransactionMapper) getTransactionFee(partialFeeStr, tipStr string) ([]structs.TransactionAmount, error) {
	var ok bool
	var amount, fee, tip *big.Int

	if partialFeeStr == "" && tipStr == "" {
		return nil, nil
	}

	if partialFeeStr != "" {
		if fee, ok = new(big.Int).SetString(partialFeeStr, 10); !ok {
			return nil, fmt.Errorf("Could not parse transaction partial fee %q", partialFeeStr)
		} else {
			amount = fee
		}
	}

	if tipStr != "" {
		if tip, ok = new(big.Int).SetString(tipStr, 10); !ok {
			return nil, fmt.Errorf("Could not parse transaction tip %q", tipStr)
		}
		if fee == nil {
			amount = tip
		} else {
			amount = new(big.Int).Add(fee, tip)
		}
	}

	textAmount, err := countCurrencyAmount(m.exp, amount.String(), m.div)
	if err != nil {
		return nil, errors.New("Could not count currency amount")
	}

	return []structs.TransactionAmount{{
		Text:     fmt.Sprintf("%s%s", textAmount.Text('f', -1), m.currency),
		Currency: m.currency,
		Numeric:  amount,
		Exp:      int32(m.exp),
	}}, nil
}

func countCurrencyAmount(exp int, value string, divider *big.Float) (*big.Float, error) {
	amount := new(big.Float)
	amount, ok := amount.SetString(value)
	if !ok {
		return nil, fmt.Errorf("Could not create big float from value %s", value)
	}

	return new(big.Float).Quo(amount, divider), nil
}
