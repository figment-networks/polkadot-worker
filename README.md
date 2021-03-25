# POLKADOT WORKER

This repository contains a worker part dedicated for polkadot transactions.

## Worker
Stateless worker is responsible for connecting with the chain, getting information, converting it to a common format and sending it back to manager.
Worker can be connected with multiple managers but should always answer only to the one that sent request.

## API
Implementation of bare requests for network.

### Client
Worker's business logic wiring of messages to client's functions.


## Installation
This system can be put together in many different ways.
This readme will describe only the simplest one worker, one manager with embedded scheduler approach.

### Compile
To compile sources you need to have go 1.14.1+ installed.

```bash
    make build
```

### Running
Worker also need some basic config:

```json
{
    "address": "0.0.0.0",
    "port": "3000",
    "http_port":"8087",
    "polkadot_proxy_addr":  "localhost:50051",
    "network": "polkadot",
    "chain_id": "mainnet",
    "managers":"0.0.0.0:8085",
    "currency": "DOT",
    "exp": "10",
}
```
where`polkadot_proxy_addr` is a http address to a running instance of [polkadot proxy](https://github.com/figment-networks/polkadothub-proxy), 
`managers` is a a comma-separated list of manager ip:port addresses that worker will connect to, `currency` is the unit of currency and `exp` is the number of decimal places required to convert planck to desired currency unit (see [here](https://wiki.polkadot.network/docs/en/learn-DOT)).


After running binary worker should successfully register itself to the manager.

## Event Types
List of currently supporter event types in polkadot-worker are (listed by modules):

balances:
- `balanceset`
- `deposit`
- `dustlost`
- `endowed`
- `reserverepatriated`
- `reserved`
- `transfer`
- `unreserved`

council:
- `approved`
- `closed`
- `disapproved`
- `executed`
- `proposed`
- `voted`

democracy:
- `cancelled`
- `delegated`
- `preimagenoted`
- `preimagereaped`
- `proposed`
- `started`
- `undelegated`

identity:
- `identitycleared`
- `identitykilled`
- `identityset`
- `judgementgiven`
- `judgementrequested`
- `judgementunrequested`
- `registraradded`
- `subidentityadded`
- `subidentiyremoved`
- `subidentityrevoked`

indices:
- `indexassigned`
- `indexfreed`
- `indexfrozen`

multisig:
- `multisigapproval`
- `multisigcancelled`
- `multisigexecuted`
- `newmultisig`

proxy:
- `announced`
- `anonymouscreated`
- `proxyexecuted`

staking:
- `bonded`
- `reward`
- `slash`

system:
- `extrinsicfailed`
- `extrinsicsuccess`
- `killedaccount`
- `newaccount`

technicalcommittee:
- `approved`
- `closed`
- `disapproved`
- `executed`
- `memberexecuted`
- `proposed`
- `voted`

tips:
- `newtip`
- `tipclosed`
- `tipclosing`
- `tipretracted`
- `tipslashed`

treasury:
- `proposed`
- `rejected`

utility:
- `batchcompleted`
- `batchinterrupted`

vesting:
- `vestingupdated`
- `vestingcompleted`