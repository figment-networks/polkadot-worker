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

```bash
    {
        "indexer_manager":  {
            "base_url": "127.0.0.1:8085",
            "host": "host.docker.internal",
            "listen_port": ":3000"
        },
        "proxy_base_url": "localhost:50051",
        },
        "worker": {
            "chain_id": "Polkadot",
            "currency": "DOT",
            "exp":      12,
            "log_level": "info",
            "network": "Polkadot",
            "version": "0.0.1",
            "host": "0.0.0.0",
            "port": ":3001"
        }
    }
```

After running binary worker should successfully register itself to the manager.

## Event Types
List of currently supporter event types in polkadot-worker are:
- DustLost
- KilledAccount
- Transfer
- BatchCompleted
- Deposit
- ExtrinsicSuccess
- ElectionProviderMultiPhase
