# GoDNS


[![Go Report Card](https://goreportcard.com/badge/github.com/extremtechniker/godns)](https://goreportcard.com/report/github.com/extremtechniker/godns)
[![Build Docker](https://github.com/extremtechniker/godns/actions/workflows/docker-build.yml/badge.svg)](https://github.com/extremtechniker/godns/actions/workflows/docker-build.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)


**GoDNS** is a lightweight, high-performance DNS server written in Go with Postgres persistence, Redis caching, CLI
management, and an optional HTTP API for CRUD operations. It supports structured logging, metrics, and JWT-secured API
access.

* * *

Features
--------

* **DNS server**:
    * Supports `A`, `AAAA`, `CNAME`, and `TXT` records.
    * Serves records from Postgres, caches them in Redis for faster access.
    * Updates cache automatically based on hit counts.
* **Persistence & caching**:
    * Postgres for persistent DNS records and metrics.
    * Redis for per-record caching.
* **CLI commands**:
    * Add or cache records.
    * Generate JWT tokens for API authentication.
* **HTTP API**:
    * Full CRUD support for records.
    * Add/remove records from cache.
    * Secured via JWT tokens.
* **Logging**:
    * Structured logging using `zap`.
    * Configurable log level and format (`json` or console).

* * *

Requirements
------------

* Go 1.25+
* Postgres 18+
* Redis 8+
* Optional: Docker Compose (for quick setup)

* * *

Installation
------------

Clone the repository:

```bash
git clone https://github.com/extremtechniker/godns.git
cd godns
```

Install dependencies:

```bash
go mod tidy
```

* * *

Docker Setup (Optional)
-----------------------

You can start Postgres and Redis quickly using the provided `compose.yml`:

```bash
docker-compose up -d
```

Defaults:

* Postgres: `root/root` user/password, database `godns`.
* Redis: `root` password, running on default port 6379.

* * *

Environment Variables
---------------------

| Variable             | Default                                                        | Description                                      |
|----------------------|----------------------------------------------------------------|--------------------------------------------------|
| `PG_URL`             | `postgres://root:root@localhost:5432/postgres?sslmode=disable` | Postgres connection string                       |
| `REDIS_ADDR`         | `localhost:6379`                                               | Redis server address                             |
| `REDIS_PASS`         | `""`                                                           | Redis password                                   |
| `DNS_LISTEN`         | `:1053`                                                        | DNS server listen address                        |
| `API_LISTEN`         | `:8080`                                                        | HTTP API listen address                          |
| `LOG_LEVEL`          | `info`                                                         | Logging level (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT`         | `""`                                                           | Logging format (`json` or empty for console)     |
| `JWT_SECRET`         | `supersecret`                                                  | Secret key for JWT authentication                |
| `MIN_HITS_FOR_CACHE` | `5`                                                            | Minimum hits required to cache record            |

* * *

CLI Usage
---------

### Root command

```bash
go run main.go --help
```

### Add DNS record

```bash
go run main.go add-record <domain> <type> <value> [ttl]
```

* Example:

```bash
go run main.go add-record example.com A 192.168.1.1 300
```

### Cache a record

```bash
go run main.go cache-record <domain> <type>
```

* Example:

```bash
go run main.go cache-record example.com A
```

### Run DNS daemon

```bash
go run main.go daemon
```

* Optional: start HTTP API by setting `API_LISTEN` env variable.

### Generate JWT token

```bash
go run main.go token [--ttl 2h]
```

* Outputs a bearer token for API authentication.
* Optional TTL argument to set token expiration.

* * *

HTTP API
--------

All API routes are protected with JWT.

### Base URL

```text
http://<server>:<API_LISTEN>
```

### Endpoints

* **POST /records** – Add or update a record (also updates cache if exists).
* **GET /records/:domain/:qtype** – Fetch a record.
* **PUT /records/:domain/:qtype** – Update a record (optional TTL).
* **DELETE /records/:domain/:qtype** – Delete a record.
* **POST /cache/:domain/:qtype** – Add a record to Redis cache.
* **DELETE /cache/:domain/:qtype** – Remove a record from Redis cache.

### Example Request

```bash
curl -X POST http://localhost:8080/records \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com","qtype":"A","value":"192.168.1.1","ttl":300}'
```

* * *

Logging
-------

* Default log level: `info`.
* Change with `LOG_LEVEL` environment variable.
* JSON format available with `LOG_FORMAT=json`.

* * *

Contributing
------------

1. Fork the repo.
2. Create a feature branch.
3. Make changes and add tests.
4. Open a pull request.

* * *

License
-------

[MIT License](./LICENSE)