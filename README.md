# MQuery

MQuery is an HTTP API server for mining language corpora using Manatee-Open engine.

## Requirements

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
8. Copy at least one corpus and its configuration (registry) into respective directories (`/var/opt/corpora/data`, `/var/opt/corpora/registry`)
9. Update corpora entries in `/opt/mquery/conf.json` file to match your installed corpora
10. start the service:
      * `systemctl start mquery-server`
      * `systemctl start mquery-worker-all.target`


## API

For the most recent API Docs, please see https://korpus.cz/mquery-test/docs/
