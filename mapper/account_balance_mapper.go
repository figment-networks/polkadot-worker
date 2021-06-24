package mapper

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/figment-networks/indexer-search/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
)

type AccountBalanceMapper struct {
	exp      int
	div      *big.Float
	currency string
}

// NewAccountBalanceMapper creates a new Account Balance mapper
func NewAccountBalanceMapper(exp int, currency string) *AccountBalanceMapper {
	return &AccountBalanceMapper{exp: exp, div: initExpDivider(int64(exp)), currency: currency}
}

// AccountBalanceMapper maps Account balance to database struct
func (m *AccountBalanceMapper) AccountBalanceMapper(accountBalanceResp *accountpb.GetByHeightResponse, height uint64) (*structs.GetAccountBalanceResponse, error) {
	if accountBalanceResp == nil || height == 0 {
		return nil, errors.New("Empty account balance response")
	}

	if amount := accountBalanceResp.Account.Free; amount != "" && amount != "0" {
		trAmount, err := getAmount(amount, m.exp, m.currency, m.div)
		if err != nil {
			return nil, fmt.Errorf("Could not create transaction free amount from value %q %s", amount, err.Error())
		}
		return &structs.GetAccountBalanceResponse{
			Height:   height,
			Balances: []structs.TransactionAmount{*trAmount},
		}, nil
	}

	return &structs.GetAccountBalanceResponse{
		Height:   height,
		Balances: nil,
	}, nil
}
