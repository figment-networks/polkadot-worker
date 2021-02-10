package celo

import (
	"context"
	"fmt"
	"math/big"

	kliento "github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/client/debug"
	"github.com/celo-org/kliento/registry"
	celoTypes "github.com/ethereum/go-ethereum/core/types"
	base "github.com/figment-networks/celo-indexer/client"
	"github.com/figment-networks/celo-indexer/client/figmentclient"
	"github.com/figment-networks/celo-indexer/utils"
	"github.com/figment-networks/celo-indexer/utils/logger"
)

type ClientIface interface {
	base.Client

	GetBlockByHeight(context.Context, int64) (*figmentclient.Block, error)
	GetTransactionsByHeight(context.Context, int64) ([]*figmentclient.Transaction, error)
}

type Client struct {
	*kliento.CeloClient
}

func New(url string) (*Client, error) {
	client, err := kliento.Dial(url)
	if err != nil {
		return nil, err
	}

	return &Client{client}, nil
}

func (c *Client) Close() {
	c.Close()
}

func (c *Client) GetBlockByHeight(ctx context.Context, h int64) (*figmentclient.Block, error) {
	var height *big.Int
	if h == 0 {
		height = nil
	} else {
		height = big.NewInt(h)
	}

	rawBlock, err := c.Eth.BlockByNumber(ctx, height)
	if err != nil {
		return nil, err
	}

	nextHeight := big.NewInt(0)
	nextHeight.Add(height, big.NewInt(1))
	nextBlockHeader, err := c.Eth.HeaderByNumber(ctx, nextHeight)
	if err != nil {
		return nil, err
	}

	extra, err := celoTypes.ExtractIstanbulExtra(nextBlockHeader)
	if err != nil {
		return nil, err
	}

	block := &figmentclient.Block{
		Hash:            rawBlock.Hash().String(),
		Height:          rawBlock.Number().Int64(),
		ParentHash:      rawBlock.ParentHash().String(),
		Time:            rawBlock.Time(),
		Size:            float64(rawBlock.Size()),
		GasUsed:         rawBlock.GasUsed(),
		Coinbase:        rawBlock.Coinbase().String(),
		Root:            rawBlock.Root().String(),
		TxHash:          rawBlock.TxHash().String(),
		RecipientHash:   rawBlock.ReceiptHash().String(),
		TotalDifficulty: rawBlock.TotalDifficulty().Uint64(),
		Extra: figmentclient.BlockExtra{
			AddedValidators:           utils.StringifyAddresses(extra.AddedValidators),
			AddedValidatorsPublicKeys: extra.AddedValidatorsPublicKeys,
			RemovedValidators:         extra.RemovedValidators,
			Seal:                      extra.Seal,
			AggregatedSeal: figmentclient.IstanbulAggregatedSeal{
				Bitmap:    extra.AggregatedSeal.Bitmap,
				Signature: extra.AggregatedSeal.Signature,
				Round:     extra.AggregatedSeal.Round.Uint64(),
			},
			ParentAggregatedSeal: figmentclient.IstanbulAggregatedSeal{
				Bitmap:    extra.ParentAggregatedSeal.Bitmap,
				Signature: extra.ParentAggregatedSeal.Signature,
				Round:     extra.ParentAggregatedSeal.Round.Uint64(),
			},
		},
		TxCount: len(rawBlock.Transactions()),
	}

	return block, nil
}

func (c *Client) GetTransactionsByHeight(ctx context.Context, h int64) ([]*figmentclient.Transaction, error) {
	var height *big.Int
	if h == 0 {
		height = nil
	} else {
		height = big.NewInt(h)
	}

	cr, err := NewContractsRegistry(c.CeloClient, height)
	if err != nil {
		return nil, err
	}
	setupErr := cr.setupContracts(ctx)

	block, err := c.Eth.BlockByNumber(ctx, height)
	if err != nil {
		return nil, err
	}

	rawTransactions := block.Transactions()

	var transactions []*figmentclient.Transaction
	for _, tx := range rawTransactions {
		txHash := tx.Hash()

		receipt, err := c.Eth.TransactionReceipt(ctx, txHash)
		if err != nil {
			return nil, err
		}

		var operations []*figmentclient.Operation

		// Internal transfers
		internalTransfers, err := c.Debug.TransactionTransfers(ctx, txHash)
		if err != nil {
			return nil, fmt.Errorf("can't run celo-rpc tx-tracer: %w", err)
		}
		operations = append(operations, c.parseFromInternalTransfers(internalTransfers)...)

		// Operations from logs
		operationsFromLogs, err := c.parseFromLogs(cr, receipt.Logs)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operationsFromLogs...)

		transaction := &figmentclient.Transaction{
			Hash:       tx.Hash().String(),
			Time:       block.Time(),
			Height:     block.Number().Int64(),
			Size:       tx.Size().String(),
			Nonce:      tx.Nonce(),
			GasPrice:   tx.GasPrice(),
			Gas:        tx.Gas(),
			GatewayFee: tx.GatewayFee(),

			Index:             receipt.TransactionIndex,
			GasUsed:           receipt.GasUsed,
			CumulativeGasUsed: receipt.CumulativeGasUsed,
			Success:           receipt.Status == celoTypes.ReceiptStatusSuccessful,
			Operations:        operations,
		}

		if tx.To() != nil {
			transaction.To = tx.To().String()
		}

		if tx.GatewayFeeRecipient() != nil {
			transaction.GatewayFeeRecipient = tx.GatewayFeeRecipient().String()
		}

		transactions = append(transactions, transaction)
	}

	return transactions, setupErr
}

func (c *Client) parseFromInternalTransfers(internalTransfers []debug.Transfer) []*figmentclient.Operation {
	var operations []*figmentclient.Operation
	for i, t := range internalTransfers {
		transfer := &figmentclient.Transfer{
			Index:   uint64(i),
			Type:    "transfer",
			From:    t.From.String(),
			To:      t.To.String(),
			Value:   t.Value,
			Success: t.Status == debug.TransferStatusSuccess,
		}

		operations = append(operations, &figmentclient.Operation{
			Name:    figmentclient.OperationTypeInternalTransfer,
			Details: transfer,
		})
	}
	return operations
}

func (c *Client) parseFromLogs(cr *contractsRegistry, logs []*celoTypes.Log) ([]*figmentclient.Operation, error) {
	var operations []*figmentclient.Operation
	for _, eventLog := range logs {
		if eventLog.Address == cr.addresses[registry.ElectionContractID] && cr.contractDeployed(registry.ElectionContractID) {
			eventName, eventRaw, _, err := cr.electionContract.TryParseLog(*eventLog)
			if err != nil {
				logger.Error(fmt.Errorf("can't parse Election event: %w", err))
			} else {
				operations = append(operations, &figmentclient.Operation{
					Name:    eventName,
					Details: eventRaw,
				})
			}

		} else if eventLog.Address == cr.addresses[registry.AccountsContractID] && cr.contractDeployed(registry.AccountsContractID) {
			eventName, eventRaw, _, err := cr.accountsContract.TryParseLog(*eventLog)
			if err != nil {
				logger.Error(fmt.Errorf("can't parse Accounts event: %w", err))
			} else {
				operations = append(operations, &figmentclient.Operation{
					Name:    eventName,
					Details: eventRaw,
				})
			}

		} else if eventLog.Address == cr.addresses[registry.LockedGoldContractID] && cr.contractDeployed(registry.LockedGoldContractID) {
			eventName, eventRaw, _, err := cr.lockedGoldContract.TryParseLog(*eventLog)
			if err != nil {
				logger.Error(fmt.Errorf("can't parse LockedGold event: %w", err))
			} else {
				operations = append(operations, &figmentclient.Operation{
					Name:    eventName,
					Details: eventRaw,
				})
			}

		} else if eventLog.Address == cr.addresses[registry.StableTokenContractID] && cr.contractDeployed(registry.StableTokenContractID) {
			eventName, eventRaw, _, err := cr.stableTokenContract.TryParseLog(*eventLog)
			if err != nil {
				logger.Error(fmt.Errorf("can't parse StableToken event: %w", err))
			} else {
				operations = append(operations, &figmentclient.Operation{
					Name:    eventName,
					Details: eventRaw,
				})
			}

		} else if eventLog.Address == cr.addresses[registry.GoldTokenContractID] && cr.contractDeployed(registry.GoldTokenContractID) {
			eventName, eventRaw, _, err := cr.goldTokenContract.TryParseLog(*eventLog)
			if err != nil {
				logger.Error(fmt.Errorf("can't parse GoldToken event: %w", err))
			} else {
				operations = append(operations, &figmentclient.Operation{
					Name:    eventName,
					Details: eventRaw,
				})
			}

		} else if eventLog.Address == cr.addresses[registry.ValidatorsContractID] && cr.contractDeployed(registry.ValidatorsContractID) {
			eventName, eventRaw, _, err := cr.validatorsContract.TryParseLog(*eventLog)
			if err != nil {
				logger.Error(fmt.Errorf("can't parse Validators event: %w", err))
			} else {
				operations = append(operations, &figmentclient.Operation{
					Name:    eventName,
					Details: eventRaw,
				})
			}

		} else if eventLog.Address == cr.addresses[registry.GovernanceContractID] && cr.contractDeployed(registry.GovernanceContractID) {
			eventName, eventRaw, _, err := cr.governanceContract.TryParseLog(*eventLog)
			if err != nil {
				logger.Error(fmt.Errorf("can't parse Governance event: %w", err))
			} else {
				operations = append(operations, &figmentclient.Operation{
					Name:    eventName,
					Details: eventRaw,
				})
			}

		}

	}
	return operations, nil
}
