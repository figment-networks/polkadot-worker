package mapper_test

import (
	"testing"

	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/stretchr/testify/suite"
)

type AccountBalanceTest struct {
	suite.Suite

	BalanceText        string
	BalanceNumeric     string
	Currency           string
	Exp                int
	Height             uint64
	AccountBalanceResp *accountpb.GetByHeightResponse
	ABMapper           *mapper.AccountBalanceMapper
}

func (a *AccountBalanceTest) SetupTest() {
	a.Currency = "DOT"
	a.Exp = 12
	a.Height = 4025307
	a.BalanceText = "0.0000000001DOT"
	a.BalanceNumeric = "100"

	a.AccountBalanceResp = &accountpb.GetByHeightResponse{
		Account: &accountpb.Account{
			Nonce:      1,
			Free:       "100",
			Reserved:   "2000",
			MiscFrozen: "30000",
			FeeFrozen:  "400000",
		},
	}

	a.ABMapper = mapper.NewAccountBalanceMapper(a.Exp, a.Currency)
}

func (a *AccountBalanceTest) TestAccountBalanceMapper_Error() {
	a.AccountBalanceResp.Account.Free = "bad"

	accountBalance, err := a.ABMapper.AccountBalanceMapper(a.AccountBalanceResp, a.Height)

	a.Require().Nil(accountBalance)
	a.Require().NotNil(err)

	a.Require().Contains(err.Error(), "Could not create transaction free amount from value \"bad\" Could not create big int from value bad")
}

func (a *AccountBalanceTest) TestAccountBalanceMapper_NoBalance() {
	a.AccountBalanceResp.Account.Free = "0"

	accountBalance, err := a.ABMapper.AccountBalanceMapper(a.AccountBalanceResp, a.Height)

	a.Require().Nil(err)

	a.Require().NotNil(accountBalance)
	a.Require().Equal(a.Height, accountBalance.Height)
	a.Require().Nil(accountBalance.Balances)
}

func (a *AccountBalanceTest) TestAccountBalanceMapper_AllOK() {
	accountBalance, err := a.ABMapper.AccountBalanceMapper(a.AccountBalanceResp, a.Height)

	a.Require().Nil(err)

	a.Require().NotNil(accountBalance)
	a.Require().Equal(a.Height, accountBalance.Height)

	a.Require().Equal(1, len(accountBalance.Balances))

	balance := accountBalance.Balances[0]
	a.Require().EqualValues(a.Exp, balance.Exp)
	a.Require().Equal(a.Currency, balance.Currency)
	a.Require().Equal(a.BalanceText, balance.Text)
	a.Require().Equal(a.BalanceNumeric, balance.Numeric.String())
}

func TestAccountBalance(t *testing.T) {
	suite.Run(t, new(AccountBalanceTest))
}
