package celo

import (
	"context"
	"errors"
	"math/big"

	kliento "github.com/celo-org/kliento/client"
	"github.com/celo-org/kliento/contracts"
	"github.com/celo-org/kliento/registry"
	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrContractNotDeployed = errors.New("contract not deployed")
)

func NewContractsRegistry(cc *kliento.CeloClient, height *big.Int) (*contractsRegistry, error) {
	reg, err := registry.New(cc)
	if err != nil {
		return nil, err
	}

	return &contractsRegistry{
		cc:     cc,
		reg:    reg,
		height: height,

		addresses: map[registry.ContractID]common.Address{},
	}, nil
}

type contractsRegistry struct {
	cc     *kliento.CeloClient
	reg    registry.Registry
	height *big.Int

	addresses map[registry.ContractID]common.Address

	reserveContract      *contracts.Reserve
	stableTokenContract  *contracts.StableToken
	validatorsContract   *contracts.Validators
	lockedGoldContract   *contracts.LockedGold
	electionContract     *contracts.Election
	accountsContract     *contracts.Accounts
	goldTokenContract    *contracts.GoldToken
	chainParamsContract  *contracts.BlockchainParameters
	epochRewardsContract *contracts.EpochRewards
	governanceContract   *contracts.Governance
}

func (cr *contractsRegistry) setupContracts(ctx context.Context, contracts ...registry.ContractID) error {
	if len(contracts) == 0 || contractIncluded(contracts, registry.ReserveContractID) {
		err := cr.setupReserveContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.StableTokenContractID) {
		err := cr.setupStableTokenContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.ValidatorsContractID) {
		err := cr.setupValidatorsContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.LockedGoldContractID) {
		err := cr.setupLockedGoldContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.ElectionContractID) {
		err := cr.setupElectionContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.AccountsContractID) {
		err := cr.setupAccountsContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.GoldTokenContractID) {
		err := cr.setupGoldTokenContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.EpochRewardsContractID) {
		err := cr.setupEpochRewardsContract(ctx)
		if err != nil {
			return err
		}
	}
	if len(contracts) == 0 || contractIncluded(contracts, registry.GovernanceContractID) {
		err := cr.setupGovernanceContract(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func contractIncluded(contracts []registry.ContractID, contractID registry.ContractID) bool {
	for _, id := range contracts {
		if id == contractID {
			return true
		}
	}
	return false
}

func (cr *contractsRegistry) setupReserveContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.ReserveContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewReserve(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.ReserveContractID] = address
	cr.reserveContract = contract

	return nil
}

func (cr *contractsRegistry) setupStableTokenContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.StableTokenContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewStableToken(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.StableTokenContractID] = address
	cr.stableTokenContract = contract

	return nil
}

func (cr *contractsRegistry) setupValidatorsContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.ValidatorsContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewValidators(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.ValidatorsContractID] = address
	cr.validatorsContract = contract

	return nil
}

func (cr *contractsRegistry) setupLockedGoldContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.LockedGoldContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewLockedGold(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.LockedGoldContractID] = address
	cr.lockedGoldContract = contract

	return nil
}

func (cr *contractsRegistry) setupElectionContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.ElectionContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewElection(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.ElectionContractID] = address
	cr.electionContract = contract

	return nil
}

func (cr *contractsRegistry) setupAccountsContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.AccountsContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewAccounts(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.AccountsContractID] = address
	cr.accountsContract = contract

	return nil
}

func (cr *contractsRegistry) setupGoldTokenContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.GoldTokenContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewGoldToken(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.GoldTokenContractID] = address
	cr.goldTokenContract = contract

	return nil
}

func (cr *contractsRegistry) setupEpochRewardsContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.EpochRewardsContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewEpochRewards(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.BlockchainParametersContractID] = address
	cr.epochRewardsContract = contract

	return nil
}

func (cr *contractsRegistry) setupGovernanceContract(ctx context.Context) error {
	address, err := cr.reg.GetAddressFor(ctx, cr.height, registry.GovernanceContractID)
	if err != nil {
		return checkErr(err)
	}
	contract, err := contracts.NewGovernance(address, cr.cc.Eth)
	if err != nil {
		return err
	}
	cr.addresses[registry.GovernanceContractID] = address
	cr.governanceContract = contract

	return nil
}

func (cr *contractsRegistry) contractDeployed(contractId registry.ContractID) bool {
	_, ok := cr.addresses[contractId]
	return ok
}

func checkErr(err error) error {
	if err == kliento.ErrContractNotDeployed {
		return ErrContractNotDeployed
	} else {
		return err
	}
}
