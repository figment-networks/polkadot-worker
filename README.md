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

## Developing Locally

First, you will need to set up a few dependencies:

1. [Install Go](https://golang.org/doc/install)
2. A Polkadot network node, and a [polkadot proxy](https://github.com/figment-networks/polkadothub-proxy) (in this example, we assume they're both running at http://127.0.0.1)
3. A running [manager](https://github.com/figment-networks/indexer-manager) instance
4. A running datastore API instance (e.g. [search](https://github.com/figment-networks/indexer-search) - this is configured with `STORE_HTTP_ENDPOINTS`)

Then, run the worker with some environment config:

> Note: if you are running multiple workers, you may need to specify a different port (e.g. `PORT=3001`) to avoid collisions

```
CHAIN_ID=mainnet \
STORE_HTTP_ENDPOINTS=http://127.0.0.1:8986/input/jsonrpc \
POLKADOT_PROXY_ADDR=127.0.0.1:50051 \
POLKADOT_NODE_ADDRS=127.0.0.1:9944 \
go run ./cmd/polkadot-worker
```

Upon success, you should see logs that look like this:

```log
{"level":"info","time":"2021-08-06T14:29:25.030-0400","msg":"polkadot-worker  (git: ) - built at "}
{"level":"info","time":"2021-08-06T14:29:25.031-0400","msg":"Self-hostname (ca00f9c1-aeca-43c9-bd86-c391629adffc) is 0.0.0.0:3000 "}
{"level":"info","time":"2021-08-06T14:29:25.031-0400","msg":"Connecting to managers (127.0.0.1:8085)"}
{"level":"info","time":"2021-08-06T14:29:25.031-0400","msg":"[API] Connecting to websocket ","host":"127.0.0.1:9944"}
{"level":"info","time":"2021-08-06T14:29:25.042-0400","msg":"[HTTP] Listening on","address":"0.0.0.0","port":"8087"}
{"level":"info","time":"2021-08-06T14:29:25.043-0400","msg":"[GRPC] Listening on","address":"0.0.0.0","port":"3000"}
```

Once the worker connects to a running [manager](https://github.com/figment-networks/indexer-manager), which runs by default at `127.0.0.1:8085`, you should see a stream registered in the logs:

```log
{"level":"debug","time":"2021-08-06T14:29:35.039-0400","msg":"Register indexer-manager client stream","streamID":"c8469fc9-663c-4efd-b2e6-599de8c4790b"}
{"level":"debug","time":"2021-08-06T14:29:35.039-0400","msg":"[GRPC] Send started "}
```
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

# Extrinsic Types
List of currently supporter extrinsic types in polkadot-worker are (listed by modules):

authorship:
- `setuncles`

babe:
- `planconfigchange`
- `reportequivocation`
- `reportequivocationunsigned`

balances:
- `forcetransfer`
- `setbalance`
- `transfer`
- `transferall`
- `transferkeepalive`

bounties:
- `acceptcurator`
- `approvebounty`
- `awardbounty`
- `claimbounty`
- `closebounty`
- `extendbountyexpiry`
- `proposecurator`
- `unassigncurator`

claims
- `attest`
- `claim`
- `claimattest`
- `mintclaim`
- `moveclaim`

council:
- `close`
- `disapproveproposal`
- `execute`
- `propose`
- `setmembers`
- `vote`

democracy:
- `blacklist`
- `cancelproposal`
- `cancelqueued`
- `cancelreferendum`
- `clearpublicproposals`
- `delegate`
- `emergencycancel`
- `enactproposal`
- `externalproposal`
- `externalpropose`
- `externalproposedefault`
- `fasttrack`
- `noteimminentpreimage`
- `noteimminentpreimageoperational`
- `notepreimage`
- `notepreimageoperational`
- `propose`
- `reappreimage`
- `removeothervote`
- `removevote`
- `second`
- `undelegate`
- `unlock`
- `vetoexternal`
- `vote`

electionprovidermultiphase:
- `setemergencyelectionresult`
- `setminimumuntrustedsource`
- `submitunsigned`

grandpa:
- `notestalled`
- `reportequivocation`
- `reportequivocationunsigned`


identity:
- `addregistrar`
- `addsub`
- `cancelrequest`
- `clearidentity`
- `killidentity`
- `providejudgement`
- `quitsub`
- `removesub`
- `renamesub`
- `requestjudgement`
- `setaccountid`
- `setfee`
- `setfields`
- `setidentity`
- `setsubs`

indices:
- `claim`
- `forcetransfer`
- `free`
- `freeze`
- `transfer`

imonline:
- `heartbeat`

multisig:
- `approveasmulti`
- `asmulti`
- `asmultithreshold1`
- `cancelasmulti`

phragmenelection:
- `cleandefunctvoters`
- `removemember`
- `removevoter`
- `renouncecandidacy`
- `submitcandidacy`


proxy:
- `addproxy`
- `announce`
- `anonymous`
- `killanonymous`
- `proxy`
- `proxyannounced`
- `rejectannouncement`
- `removeannouncement`
- `removeproxies`
- `removeproxy`

scheduler:
- `cancel`
- `cancelnamed`
- `schedule`
- `scheduleafter`
- `schedulenamed`
- `schedulenamedafter`

session:
- `purgekeys`
- `setkeys`

staking:
- `bond`
- `bondextra`
- `canceldeferredslash`
- `chill`
- `chillother`
- `forcenewera`
- `forceneweraalways`
- `forcenoeras`
- `forceunstake`
- `increasevalidatorcount`
- `kick`
- `nominate`
- `payoutstakers`
- `reapstash`
- `scalevalidatorcount`
- `setcontroller`
- `sethistorydepth`
- `setinvulnerables`
- `setvalidatorcount`
- `unbond`
- `updatestakinglimits`
- `validate`
- `withdrawunbonded`

system:
- `fillblock`
- `killprefix`
- `killstorage`
- `remark`
- `remarkwithevent`
- `setchangestrieconfig`
- `setcode`
- `setcodewithoutchecks`
- `setheappages`
- `setstorage`

technicalcommittee:
- `close`
- `disapproveproposal`
- `execute`
- `propose`
- `setmembers`
- `vote`

technicalmembership:
- `addmember`
- `changekey`
- `clearprime`
- `removemember`
- `resetmembers`
- `setprime`
- `swapmember`

timestamp:
- `set`

tips:
- `closetip`
- `reportawesome`
- `retracttip`
- `slashtip`
- `tip`
- `tipnew`

treasury:
- `approveproposal`
- `proposespend`
- `rejectproposal`

utility:
- `asderivative`
- `batch`
- `batchall`

vesting:
- `forcevestedtransfer`
- `vest`
- `vestother`
- `vestedtransfer`