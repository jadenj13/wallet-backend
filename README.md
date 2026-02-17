# wallet-backend

A wallet backend consisting of a REST API and a set of TSS (Threshold Signature Scheme) party nodes that perform distributed key generation and signing.

## Architecture

### API

The API service is the main entrypoint for clients. It exposes wallet operations over HTTP and uses Redis for state management.

**Routes:**

- `POST /wallet` — initialize a new wallet
- `POST /wallet/sign` — request a signature

### TSS Parties

The party nodes implement a 2-of-3 threshold signature scheme using the `tss-lib` library. Each party holds a share of the private key and communicates with its peers to collaboratively generate keys and produce signatures without any single party ever holding the full key.

**Routes:**

- `POST /keygen` — initiate distributed key generation
- `POST /sign` — initiate a signing round
- `POST /message` — receive a TSS protocol message from a peer
- `GET /pubkey` — return the shared public key

## Running

### Prerequisites

- Docker and Docker Compose

### Start all services

```sh
docker compose up --build
```

This starts:

| Service   | Port |
|-----------|------|
| API       | 3000 |
| Party 1   | 3001 |
| Party 2   | 3002 |
| Party 3   | 3003 |
| Redis     | 6379 |

### Run locally without Docker

Requires Go 1.25+ and a running Redis instance.

**API:**

```sh
export REDIS_ADDR=localhost:6379
go run ./cmd/api
```

**Party (run each in a separate terminal):**

```sh
export PARTY_ID=1
export PEERS='["http://localhost:3001","http://localhost:3002"]'
export SERVER_PORT=3000
go run ./cmd/tss
```

Repeat for parties 2 and 3, adjusting `PARTY_ID`, `PEERS`, and `SERVER_PORT` accordingly.
