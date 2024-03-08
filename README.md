# MQuery - an HTTP API server for mining corpora using Manatee-Open engine

## API

### Concordance

:orange_circle: `GET /concordance/[corpus ID]`

Show a concordance in a "sentence" mode based on provided query. Positional attributes
in the output depend on corpus configuration.

URL arguments:

* `q` - a Manatee CQL query
* `subcorpus` - an ID of a subcorpus (which is defined in MQuery configuration)

### Frequency information

:orange_circle: `GET /text-types-overview/[corpus ID]`

Provide basic overview of frequencies of a searched expression based on different text types.

URL arguments:

* `q` - a Manatee CQL query
*

:orange_circle: `GET /freqs/[corpus ID]`


:orange_circle: `GET /freqs2/[corpus ID]`


:orange_circle: `GET /text-types/[corpus ID]`


:orange_circle: `GET /text-types2/[corpus ID]`


### Collocation profile

:orange_circle: `GET /collocs/[corpus ID]`



