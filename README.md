# MQuery

MQuery is an HTTP API server for mining language corpora using Manatee-Open engine. Unlike other Manatee-based solutions, MQuery uses more fine-tuned C bindings without relying on SWIG, and naturally leverages a worker queue architecture for efficient query processing and scalability.

## Running with Docker (Easiest Method)

The simplest way to run MQuery is using Docker Compose, which automatically sets up the server, worker, and Redis:

### Prerequisites

* [Docker](https://docs.docker.com/get-docker/)
* [Docker Compose](https://docs.docker.com/compose/install/)

### Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/czcorpus/mquery.git
   cd mquery
   ```

2. Create a Docker configuration file `conf-docker.json` based on `conf.sample.json`:
   ```bash
   cp conf.sample.json conf-docker.json
   ```

3. Edit `conf-docker.json` to match your setup:
   * Set `listenAddress` to `0.0.0.0` (to accept connections from outside the container)
   * Set `listenPort` to `8989`
   * Set Redis host to `redis` (the service name in docker-compose.yml)
   * Configure your corpora paths:
     * `registryDir`: `/var/lib/manatee/registry`
     * `splitCorporaDir`: `/var/lib/manatee/split`

4. Place your corpus data and registry files in directories that will be mounted:
   * The docker-compose setup creates volumes for corpus data at `/var/lib/manatee`
   * You can modify the volume mounts in `docker-compose.yml` to point to your existing corpus directories

5. Start the services:
   ```bash
   docker-compose up -d
   ```

6. Access the API at `http://localhost:8989`

### Docker Architecture

The Docker Compose setup includes:
* **mquery-server**: HTTP API server (port 8989)
* **mquery-worker**: Background worker for processing corpus queries
* **redis**: Redis database for job queuing and results caching

### Managing the Services

* View logs: `docker-compose logs -f`
* Stop services: `docker-compose down`
* Rebuild after code changes: `docker-compose up -d --build`

## Manual Installation

If you prefer to install MQuery manually without Docker:

### Requirements

* a working Linux server with installed [Manatee-open](https://nlp.fi.muni.cz/trac/noske) library
* [Redis](https://redis.io/) database
* [Go](https://go.dev/)  language compiler and tools
* (optional) an HTTP proxy server (Nginx, Apache, ...)


## How to install

1. Install `Go` language environment, either via a package manager or manually from Go [download page](https://go.dev/dl/)
   1. make sure `/usr/local/go/bin` and `~/go/bin` are in your `$PATH` so you can run any installed Go tools without specifying a full path
2. Install Manatee-open from the [download page](https://nlp.fi.muni.cz/trac/noske). No specific language bindings are required.
   1. `configure --with-pcre --disable-python && make && sudo make install && sudo ldconfig`
3. Get MQuery sources (`git clone --depth 1 https://github.com/czcorpus/mquery.git`)
4. Run `./configure`
5. Run `make`
6. Run `make install`
      * the application will be installed in `/opt/mquery`
      * for data and registry, `/var/opt/corpora/data` and `/var/opt/corpora/registry` directories will be created
      * systemd services `mquery-server.service` and `mquery-worker-all.target` will be created
7. Copy at least one corpus and its configuration (registry) into respective directories (`/var/opt/corpora/data`, `/var/opt/corpora/registry`)
8. Update corpora entries in `/opt/mquery/conf.json` file to match your installed corpora
9. start the service:
      * `systemctl start mquery-server`
      * `systemctl start mquery-worker-all.target`


## Authentication

MQuery supports optional token-based authentication via a configurable HTTP header. When enabled, every request must include the header with a valid token.

Relevant configuration fields in `conf.json`:

```json
{
  "authHeaderName": "X-API-Key",
  "authTokens": [
    "sha256:a3f1c8d2...",
    "sha256:9e107d9d..."
  ],
  "localNetworks": [
    "127.0.0.0/8",
    "192.168.1.0/24"
  ],
  "knownProxies": [
    "192.168.1.10"
  ]
}
```

Tokens in `authTokens` can be stored either as **plaintext** (not recommended) or as **SHA-256 hashes** prefixed with `sha256:` (recommended).

### Generating a hashed token

1. Choose a secret token (use a long random string):
   ```bash
   openssl rand -hex 32
   # example output: 4a7b9c2e1f3d8a6b...
   ```

2. Hash it for storage in `conf.json`:
   ```bash
   echo -n "your-secret-token" | sha256sum | awk '{print "sha256:" $1}'
   # output: sha256:a3f1c8d2...
   ```

3. Paste the `sha256:...` value into `authTokens` in your config.

Clients send the original (unhashed) token in the configured header:
```
X-API-Key: your-secret-token
```

Requests from IPs within any `localNetworks` CIDR range are exempt from auth token checks, provided the source IP is not also listed in `knownProxies`. If `localNetworks` is not set, only the exact `listenAddress` is treated as local.

If a reverse proxy shares an IP with a local network (e.g. runs on the same host), add its IP to `knownProxies` to ensure its forwarded requests still require auth.

## API

For the most recent API Docs, please see https://korpus.cz/mquery-test/docs/
