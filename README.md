# MQuery

MQuery is an HTTP API server for mining language corpora using Manatee-Open engine.

## API

Note: all the responses are in JSON

### General information

:orange_circle: `GET /openapi`

Show OpenAPI-compatible specification of the API

:orange_circle: `GET /privacy-policy`

Show privacy policy information (if defined)

### Corpora information

:orange_circle: `GET /info/[corpus ID]?[args...]`

Show a corpus information.

URL arguments:

* `lang` - a  ISO 639-1 code of the language client wants the description in. In case the language is not found or in case the code is omitted, `en` version is returned.

Response:

```ts
{
    corpus: {
        corpname:string;
        size:number; // number of tokens
        description:string; // a localized description (if available, otherwise the `en` version)
        flags:Array<string>; // a list of keywords characterizing the corpus
        attrList:Array<{
            name:string;
            size:number; // number of unique values
            description?:string; // a description of the attribute
        }>;
        structList:Array<{
            name:string;
            size:number; // number of occurences in data
            description?:string; // a description of the attribute
        }>;
        webUrl?:string;
        citationInfo:unknown; // currently unused
    };
    locale:string; // locale of the response (i.e. not related to corpus data)
}
```

:orange_circle: `GET /corplist?[args...]`

Shows a list of corpora with their basic properties.

URL arguments:

* `lang` - a  ISO 639-1 code of the language client wants the description in. In case the language is not found or in case the code is omitted, `en` version is returned.

Response:

```ts
{
    corpora: Array<{
        id:string;
        fullName:string;
        description:string;
        flags:Array<string>;
        subcorpora:Array<{
            id:string;
            description:string;
        }>;
    }>;
    locale:string; // locale of the response
}
```

### Concordance

:orange_circle: `GET /concordance/[corpus ID]?[args...]`

Show a concordance in a "sentence" mode based on provided query. Positional attributes
in the output depend on corpus configuration.

URL arguments:

* `q` - a Manatee CQL query
* `subcorpus` - an ID of a subcorpus (which is defined in MQuery configuration)

Response:

```ts
{
    lines:Array<{
        text:{
            word: string; // the `word` value (main text attribute)
            attrs: {[key:string]:string}; // positional attributes and their respective values
            strong: boolean; // emphasis flag
        },
        ref:string; // a KWIC token ID
    }>;
    concSize:number;
    resultType:'conc';
    error?:string; // if empty, the key is not present
}
```

### Frequency information

:orange_circle: `GET /text-types-overview/[corpus ID]?[args...]`

Provide basic overview of frequencies of a searched expression based on different text types.

URL arguments:

* `q` - a Manatee CQL query
* `subcorpus` - an ID of a subcorpus (which is defined in MQuery configuration)

Response:

```ts
{
    concSize:number;
    corpusSize:number;
    searchSize:number; // TODO unfinished, please do not use
    freqs:{
        [attr:string]:Array<{
            word:string;
            freq:number;
            norm:number;
            ipm:nmber;
        }>
    };
    examplesQueryTpl?:string;
    resultType:'freqTT';
}

```

:orange_circle: `GET /freqs/[corpus ID]?[args...]`

Calculate a frequency distribution for the searched term (KWIC).

URL arguments:

* `q` - a Manatee CQL query
* `subcorpus` - an ID of a subcorpus (which is defined in MQuery configuration)
* `fcrit` - a Manatee freq. criterion (e.g. `tag 0~0>0` (see [SketchEngine docs](https://www.sketchengine.eu/documentation/methods-documentation/#freqs))).
  * if omitted `lemma 0~0>0` is used
* `maxItems` - this sets the maximum number of result items
* `flimit` - minimum frequency of items to be included in the result set
* `within` - :exclamation: deprecated - use `subcorpus` instead

Response:

```ts
{
    concSize:number;
    corpusSize:number;
    searchSize:number; // TODO unfinished, please do not use
    fcrit:string; // applied Manatee freq. criterion
    freqs:Array<{
        word:string;
        freq:number; // absolute freq.
        norm:number; // a text size we calculate relative freqs. against (typically, a corpus size)

    }>;
    resultType:'freqs';
}
```


:orange_circle: `GET /freqs2/[corpus ID]`

This is a parallel variant of `freqs2` which calculates frequencies on smaller chunks and merges
them together. It is most suitable for larger corpora.


:orange_circle: `GET /text-types/[corpus ID]?[args...]`

Calculate frequencies of all the values of a requested structural attribute found in structures
matching required query (e.g. all the authors found in `&lt;doc author="..."&gt;`)

URL arguments:

* `q` - a Manatee CQL query
* `subcorpus` - an ID of a subcorpus (which is defined in MQuery configuration)
* `attr` - a structural attribute (e.g. `doc.pubyear`, `text.author`,...)


Response:

```ts
{
    concSize:number;
    corpusSize:number;
    searchSize:number; // actual searched data size - applies for subc., TODO unfinished, please do not use
    fcrit:string; // applied Manatee freq. criterion
    freqs:Array<{
        word:string;
        freq:number; // absolute freq.
        norm:number; // a text size we calculate relative freqs. against (typically, a corpus size)

    }>;
    resultType:'freqs';
}
```


:orange_circle: `GET /text-types2/[corpus ID]?[args...]`

This is a parallel variant of `text-types2` which calculates frequencies on smaller chunks and merges
them together. It is most suitable for larger corpora.


### Collocation profile

:orange_circle: `GET /collocations/[corpus ID]?[args...]`


Calculate a defined collocation profile of a searched expression. Values are sorted in descending order
by their coll. score.

URL arguments:

* `q` - a Manatee CQL query
* `subcorpus` - an ID of a subcorpus (which is defined in MQuery configuration)
* `measure`  - a collocation measure. If omitted, `logDice` is used. The available values are:
  * `absFreq`
  * `logLikelihood`
  * `logDice`
  * `minSensitivity`
  * `mutualInfo`
  * `mutualInfo3`
  * `mutualInfoLogF`
  * `relFreq`
  * `tScore`
* `srchLeft` - left range for candidates searching (`0` is KWIC, values `< 0` are on the left side of the KWIC, values `> 0` are to the right of the KWIC). The argument can be omitted in which case `-5` is used
* `srchRight` - right range for candidates searching (the meaning of concrete values is the same as in `srchLeft`). The argument can be omitted in which case `-5` is used.
* `minCollFreq` - the minimum frequency that a collocate must have in the searched range. The argument is optional with default value of `3`
* `maxItems`- maximum number of result items. The argument is optional with default value of `20`

example req:

```
/collocations/intercorp_v13ud_cs?q=[lemma=%22podoba%22]&subcorpus=core&measure=mutualInfo&srchLeft=3&maxItems=5
```

Response:

```ts
{
    corpusSize:number;
    searchSize:number; // actual searched data size - applies for subc., TODO unfinished, please do not use
    concSize:number;
    measure:string; // applied measure
    resultType:'coll';
    srchRange:[number, number];
    colls:Array<{
        word:string;
        score:number;
        freq:number;
    }>;
}
```
