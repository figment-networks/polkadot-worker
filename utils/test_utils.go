package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/figment-networks/indexing-engine/structs"

	"github.com/stretchr/testify/suite"
)

func ValidateTransactions(sut *suite.Suite, transaction, expected structs.Transaction) {
	sut.Require().Equal(expected.BlockHash, transaction.BlockHash)
	sut.Require().EqualValues(expected.Height, transaction.Height)

	sut.Require().Equal(expected.Hash, transaction.Hash)
	sut.Require().Equal(expected.ChainID, transaction.ChainID)
	sut.Require().Equal(expected.Time.Unix(), transaction.Time.Unix())
	sut.Require().Equal(expected.Version, transaction.Version)
	sut.Require().Equal(expected.HasErrors, transaction.HasErrors)

	if transaction.Fee != nil && transaction.Fee[0].Text != "" {
		validateAmount(sut, &transaction.Fee[0], &expected.Fee[0])
	}

	sut.Require().Len(transaction.Events, len(expected.Events))

	for i, event := range transaction.Events {
		exEvent := expected.Events[i]
		sut.Require().Equal(exEvent.Kind, event.Kind)
		sut.Require().Equal(exEvent.Type, event.Type)
		sut.Require().Equal(exEvent.Module, event.Module)

		sut.Require().Len(event.Sub, len(exEvent.Sub))
		for j, e := range event.Sub {
			exEv := exEvent.Sub[j]
			sut.Require().Equal(exEv.ID, e.ID)

			sut.Require().Equal(exEv.Type, e.Type)

			sut.Require().Equal(exEv.Module, e.Module)
			sut.Require().Equal(exEv.Nonce, e.Nonce)
			sut.Require().Equal(exEv.Completion.Unix(), e.Completion.Unix())

			if a, ok := e.Amount["0"]; ok {
				exAmount, ok := exEv.Amount["0"]
				sut.Require().True(ok)
				validateAmount(sut, &a, &exAmount)
			}

			if accounts, ok := e.Node["versions"]; ok {
				exAccounts, ok := exEv.Node["versions"]
				sut.Require().True(ok)
				sut.Require().Equal(exAccounts, accounts)
			}

			if account, ok := e.Node["sender"]; ok {
				exSender, ok := exEv.Node["sender"]
				sut.Require().True(ok)
				sut.Require().Equal(exSender, account)
			}

			if account, ok := e.Node["recipient"]; ok {
				exRecipient, ok := exEv.Node["recipient"]
				sut.Require().True(ok)

				accountID := account[0].ID

				sut.Require().Equal(exRecipient[0].ID, accountID)

				transfers := e.Transfers[accountID]
				exTransfers := exEv.Transfers[accountID]

				validateAmount(sut, &transfers[0].Amounts[0], &exTransfers[0].Amounts[0])
			}
		}
	}
}

func validateAmount(sut *suite.Suite, amount, expected *structs.TransactionAmount) {
	if amount == nil {
		return
	}

	sut.Require().Equal(expected.Currency, amount.Currency)
	sut.Require().Equal(expected.Exp, amount.Exp)
	sut.Require().Equal(expected.Numeric, amount.Numeric)
	sut.Require().Equal(expected.Text, amount.Text)
}

func ReadFile(sut suite.Suite, dir string, dest interface{}) {
	ddrFile, err := os.Open(dir)
	sut.Require().Nil(err)
	defer ddrFile.Close()

	b, err := ioutil.ReadAll(ddrFile)
	sut.Require().Nil(err)
	sut.Require().Nil(json.Unmarshal(b, dest))
}
