package mapper

import "fmt"

func getErrorMsg(index, code float64) string {
	fmt.Println("mapping error: ", index, code)
	switch index {
	case 0:
		return getSystemErrorMsg(code)
	case 1:
		return getSchedulerErrorMsg(code)
	case 5:
		return getBalancesErrorMsg(code)
	case 6:
		return getAuthorshipErrorMsg(code)
	case 7:
		return getStakingErrorMsg(code)
	case 9:
		return getSessionErrorMsg(code)
	case 11:
		return getGrandpaErrorMsg(code)
	case 12:
		return getImOnlineErrorMsg(code)
	case 14:
		return getDemocracyErrorMsg(code)
	case 15:
		return getCouncilOrTechnicalCommitteeErrorMsg(code, "council")
	case 16:
		return getCouncilOrTechnicalCommitteeErrorMsg(code, "technical committee")
	case 17:
		return getElectionsPhragmenErrorMsg(code)
	case 19:
		return getTreasuryErrorMsg(code)
	case 24:
		return getClaimsErrorMsg(code)
	case 25:
		return getVestingErrorMsg(code)
	case 28:
		return getIdentityErrorMsg(code)
	case 29:
		return getProxyErrorMsg(code)
	case 30:
		return getMultisigErrorMsg(code)
	default:
		return fmt.Sprintf("Unknown error index %f.", index)
	}
}

func getSystemErrorMsg(code float64) string {
	switch code {
	case 0:
		return "The name of specification does not match between the current runtime and the new runtime."
	case 1:
		return "The specification version is not allowed to decrease between the current runtime and the new runtime."
	case 2:
		return `Failed to extract the runtime version from the new runtime. Either calling "Core_version" or decoding "RuntimeVersion" failed.`
	case 3:
		return "Suicide called when the account has non-default composite data."
	case 4:
		return "There is a non-zero reference count preventing the account from being purged."
	default:
		return fmt.Sprintf("Unknown system error code %f.", code)
	}
}

func getSchedulerErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Failed to schedule a call."
	case 1:
		return "Cannot find the scheduled call."
	case 2:
		return "Given target block number is in the past."
	case 3:
		return "Reschedule failed because it does not change scheduled time."
	default:
		return fmt.Sprintf("Unknown scheduler error code %f.", code)
	}
}

func getBalancesErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Vesting balance too high to send value."
	case 1:
		return "Account liquidity restrictions prevent withdrawal."
	case 2:
		return "Got an overflow after adding."
	case 3:
		return "Balance too low to send value."
	case 4:
		return "Value too low to create account due to existential deposit."
	case 5:
		return "Transfer/payment would kill account."
	case 6:
		return "A vesting schedule already exists for this account."
	case 7:
		return "Beneficiary account must pre-exist."
	default:
		return fmt.Sprintf("Unknown balances error code %f.", code)
	}
}

func getAuthorshipErrorMsg(code float64) string {
	switch code {
	case 0:
		return "The uncle parent not in the chain."
	case 1:
		return "Uncles already set in the block."
	case 2:
		return "Too many uncles."
	case 3:
		return "The uncle is genesis."
	case 4:
		return "The uncle is too high in chain."
	case 5:
		return "The uncle is already included."
	case 6:
		return "The uncle isn't recent enough to be included."
	default:
		return fmt.Sprintf("Unknown authorship error code %f.", code)
	}
}

func getStakingErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Not a controller account."
	case 1:
		return "Not a stash account."
	case 2:
		return "Stash is already bonded."
	case 3:
		return "Controller is already paired."
	case 4:
		return "Targets cannot be empty."
	case 5:
		return "Duplicate index."
	case 6:
		return "Slash record index out of bounds."
	case 7:
		return "Can not bond with value less than minimum balance."
	case 8:
		return "Can not schedule more unlock chunks."
	case 9:
		return "Can not rebond without unlocking chunks."
	case 10:
		return "Attempting to target a stash that still has funds."
	case 11:
		return "Invalid era to reward."
	case 12:
		return "Invalid number of nominations."
	case 13:
		return "Items are not sorted and unique."
	case 14:
		return "Rewards for this era have already been claimed for this validator."
	case 15:
		return "The submitted result is received out of the open window."
	case 16:
		return "The submitted result is not as good as the one stored on chain."
	case 17:
		return "The snapshot data of the current window is missing."
	case 18:
		return "Incorrect number of winners were presented."
	case 19:
		return "One of the submitted winners is not an active candidate on chain (index is out of range in snapshot)."
	case 20:
		return "Error while building the assignment type from the compact. This can happen if an index is invalid, or if the weights overflow."
	case 21:
		return "One of the submitted nominators is not an active nominator on chain."
	case 22:
		return "One of the submitted nominators has an edge to which they have not voted on chain."
	case 23:
		return "One of the submitted nominators has an edge which is submitted before the last non-zero slash of the target."
	case 24:
		return "A self vote must only be originated from a validator to ONLY themselves."
	case 25:
		return "The submitted result has unknown edges that are not among the presented winners."
	case 26:
		return "The claimed score does not match with the one computed from the data."
	case 27:
		return "The election size is invalid."
	case 28:
		return "The call is not allowed at the given time due to restrictions of election period."
	case 29:
		return "Incorrect previous history depth input provided."
	case 30:
		return "Incorrect number of slashing spans provided."
	default:
		return fmt.Sprintf("Unknown staking error code %f.", code)
	}
}

func getSessionErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Invalid ownership proof."
	case 1:
		return "No associated validator ID for account."
	case 2:
		return "Registered duplicate key."
	case 3:
		return "No keys are associated with this account."
	default:
		return fmt.Sprintf("Unknown session error code %f.", code)
	}
}

func getGrandpaErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Attempt to signal GRANDPA pause when the authority set isn't live (either paused or already pending pause)."
	case 1:
		return "Attempt to signal GRANDPA resume when the authority set isn't paused (either live or already pending resume)."
	case 2:
		return "Attempt to signal GRANDPA change with one already pending."
	case 3:
		return "Cannot signal forced change so soon after last."
	case 4:
		return "A key ownership proof provided as part of an equivocation report is invalid."
	case 5:
		return "An equivocation proof provided as part of an equivocation report is invalid."
	case 6:
		return "A given equivocation report is valid but already previously reported."
	default:
		return fmt.Sprintf("Unknown grandpa error code %f.", code)
	}
}

func getImOnlineErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Non existent public key."
	case 1:
		return "Duplicated heartbeat."
	default:
		return fmt.Sprintf("Unknown ImOnline error code %f.", code)
	}
}

func getDemocracyErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Value too low."
	case 1:
		return "Proposal does not exist."
	case 2:
		return "Unknown index."
	case 3:
		return "Cannot cancel the same proposal twice."
	case 4:
		return "Proposal already made."
	case 5:
		return "Proposal still blacklisted."
	case 6:
		return "Next external proposal not simple majority."
	case 7:
		return "Invalid hash."
	case 8:
		return "No external proposal."
	case 9:
		return "Identity may not veto a proposal twice."
	case 10:
		return "Not delegated."
	case 11:
		return "Preimage already noted."
	case 12:
		return "Not imminent."
	case 13:
		return "Too early."
	case 14:
		return "Imminent."
	case 15:
		return "Preimage not found."
	case 16:
		return "Vote given for invalid referendum."
	case 17:
		return "Invalid preimage."
	case 18:
		return "No proposals waiting."
	case 19:
		return "The target account does not have a lock."
	case 20:
		return "The lock on the account to be unlocked has not yet expired."
	case 21:
		return "The given account did not vote on the referendum."
	case 22:
		return "The actor has no permission to conduct the action."
	case 23:
		return "The account is already delegating."
	case 24:
		return "An unexpected integer overflow occurred."
	case 25:
		return "An unexpected integer underflow occurred."
	case 26:
		return "Too high a balance was provided that the account cannot afford."
	case 27:
		return "The account is not currently delegating."
	case 28:
		return `The account currently has votes attached to it and the operation cannot succeed until these are removed, either through "unvote" or "reap_vote".`
	case 29:
		return "The instant referendum origin is currently disallowed."
	case 30:
		return "Delegation to oneself makes no sense."
	case 31:
		return "Invalid upper bound."
	case 32:
		return "Maximum number of votes reached."
	case 33:
		return "The provided witness data is wrong."
	case 34:
		return "Maximum number of proposals reached."
	default:
		return fmt.Sprintf("Unknown democracy error code %f.", code)
	}
}

func getCouncilOrTechnicalCommitteeErrorMsg(code float64, str string) string {
	switch code {
	case 0:
		return "Account is not a member."
	case 1:
		return "Duplicate proposals not allowed."
	case 2:
		return "Proposal must exist."
	case 3:
		return "Mismatched index."
	case 4:
		return "Duplicate vote ignored."
	case 5:
		return "Members are already initialized!"
	case 6:
		return "The close call was made too early, before the end of the voting."
	case 7:
		return `There can only be a maximum of "MaxProposals" active proposals.`
	case 8:
		return "The given weight bound for the proposal was too low."
	case 9:
		return "The given length bound for the proposal was too low."
	default:
		return fmt.Sprintf("Unknown %s error code %f.", str, code)
	}
}

func getElectionsPhragmenErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Cannot vote when no candidates or members exist."
	case 1:
		return "Must vote for at least one candidate."
	case 2:
		return "Cannot vote more than candidates."
	case 3:
		return "Cannot vote more than maximum allowed."
	case 4:
		return "Cannot vote with stake less than minimum balance."
	case 5:
		return "Voter can not pay voting bond."
	case 6:
		return "Must be a voter."
	case 7:
		return "Cannot report self."
	case 8:
		return "Duplicated candidate submission."
	case 9:
		return "Member cannot re-submit candidacy."
	case 10:
		return "Runner cannot re-submit candidacy."
	case 11:
		return "Candidate does not have enough funds."
	case 12:
		return "Not a member."
	case 13:
		return "The provided count of number of candidates is incorrect."
	case 14:
		return "The provided count of number of votes is incorrect."
	case 15:
		return `The renouncing origin presented a wrong "Renouncing" parameter.`
	case 16:
		return "Prediction regarding replacement after member removal is wrong."
	default:
		return fmt.Sprintf("Unknown council error code %f.", code)
	}
}

func getTreasuryErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Proposer's balance is too low."
	case 1:
		return "No proposal or bounty at that index."
	case 2:
		return "The reason given is just too big."
	case 3:
		return "The tip was already found/started."
	case 4:
		return "The tip hash is unknown."
	case 5:
		return "The account attempting to retract the tip is not the finder of the tip."
	case 6:
		return "The tip cannot be claimed/closed because there are not enough tippers yet."
	case 7:
		return "The tip cannot be claimed/closed because it's still in the countdown period."
	case 8:
		return "The bounty status is unexpected."
	case 9:
		return "Require bounty curator."
	case 10:
		return "Invalid bounty value."
	case 11:
		return "Invalid bounty fee."
	case 12:
		return "A bounty payout is pending. To cancel the bounty, you must unassign and slash the curator."
	default:
		return fmt.Sprintf("Unknown council error code %f.", code)
	}
}

func getClaimsErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Invalid Ethereum signature."
	case 1:
		return "Ethereum address has no claim."
	case 2:
		return "Account ID sending tx has no claim."
	case 3:
		return "There's not enough in the pot to pay out some unvested amount. Generally implies a logic error."
	case 4:
		return "A needed statement was not included."
	case 5:
		return "The account already has a vested balance."
	default:
		return fmt.Sprintf("Unknown council error code %f.", code)
	}
}

func getVestingErrorMsg(code float64) string {
	switch code {
	case 0:
		return "The account given is not vesting."
	case 1:
		return "An existing vesting schedule already exists for this account that cannot be clobbered."
	case 2:
		return "Amount being transferred is too low to create a vesting schedule."
	default:
		return fmt.Sprintf("Unknown council error code %f.", code)
	}
}

func getIdentityErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Too many subs-accounts."
	case 1:
		return "Account isn't found."
	case 2:
		return "Account isn't named."
	case 3:
		return "Empty index."
	case 4:
		return "Fee is changed."
	case 5:
		return "No identity found."
	case 6:
		return "Sticky judgement."
	case 7:
		return "Judgement given."
	case 8:
		return "Invalid judgement."
	case 9:
		return "The index is invalid."
	case 10:
		return "The target is invalid."
	case 11:
		return "Too many additional fields."
	case 12:
		return "Maximum amount of registrars reached. Cannot add any more."
	case 13:
		return "Account ID is already named."
	case 14:
		return "Sender is not a sub-account."
	case 15:
		return "Sub-account isn't owned by sender."
	default:
		return fmt.Sprintf("Unknown council error code %f.", code)
	}
}

func getProxyErrorMsg(code float64) string {
	switch code {
	case 0:
		return "There are too many proxies registered or too many announcements pending."
	case 1:
		return "Proxy registration not found."
	case 2:
		return "Sender is not a proxy of the account to be proxied."
	case 3:
		return "A call which is incompatible with the proxy type's filter was attempted."
	case 4:
		return "Account is already a proxy."
	case 5:
		return "Call may not be made by proxy because it may escalate its privileges."
	case 6:
		return "Announcement, if made at all, was made too recently."
	default:
		return fmt.Sprintf("Unknown council error code %f.", code)
	}
}

func getMultisigErrorMsg(code float64) string {
	switch code {
	case 0:
		return "Threshold must be 2 or greater."
	case 1:
		return "Call is already approved by this signatory."
	case 2:
		return "Call doesn't need any (more) approvals."
	case 3:
		return "There are too few signatories in the list."
	case 4:
		return "There are too many signatories in the list."
	case 5:
		return "The signatories were provided out of order; they should be ordered."
	case 6:
		return "The sender was contained in the other signatories; it shouldn't be."
	case 7:
		return "Multisig operation not found when attempting to cancel."
	case 8:
		return "Only the account that originally created the multisig is able to cancel it."
	case 9:
		return "No timepoint was given, yet the multisig operation is already underway."
	case 10:
		return "A different timepoint was given to the multisig operation that is underway."
	case 11:
		return "A timepoint was given, yet no multisig operation is underway."
	case 12:
		return "The maximum weight information provided was too low."
	case 13:
		return "The data to be stored is already stored."
	default:
		return fmt.Sprintf("Unknown council error code %f.", code)
	}
}
