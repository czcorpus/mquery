# mymquery – terminal UI for MQuery

A Bubble Tea based TUI client for the MQuery HTTP API. It is a pure HTTP
client: it uses only the public GET endpoints, no CGO and no `mquery/*`
server packages. Response types are taken from `mquery-common` where
possible.

```
go build -o mymquery ./cmd/mymquery
./mymquery -api http://127.0.0.1:8082
```

## Layout

```
cmd/mymquery/
  main.go          flags, program start
  api/             thin HTTP client (one file per API area)
    client.go      base client, error decoding, /corplist, /info
    conc.go        /concordance
  ui/
    app.go         root model: screen routing, header/footer, shared session
    feature.go     Feature registry + ResultView interface
    styles.go      lipgloss styles
    menu.go        function menu
    corpsel.go     corpus selection (filterable list)
    query.go       query input
    conc.go        concordance result view
```

Every function is a `Feature` with its own `ResultView`. The app shell
(menu → corpus → query → result) stays the same for all of them, so
adding a function means adding `api/<x>.go`, `ui/<x>.go` and one
registry entry in `ui/feature.go`.

Session state is kept across screens: the selected corpus and the last
query for each feature.

## Milestones

### M0 – core shell ✅
- [x] menu with all functions (unimplemented ones are shown but disabled)
- [x] corpus selection from `/corplist` (filterable with `/`)
- [x] query input (CQL; prefilled with the last query for the feature)
- [x] header showing function / corpus / query, contextual footer help
- [x] HTTP errors shown inline (MQuery's `{"error": ...}` is decoded)

### M1 – concordance (`GET /concordance/:corpusId`) ✅ (first iteration)
- [x] KWIC layout: right-aligned left context, highlighted KWIC, right context
- [x] selectable line cursor, scrolling, paging (`rowsOffset`/`maxRows`)
- [x] `enter` opens a detail of the line: all positional attributes of
      every token in a table, text properties and ref
- [x] `a` opens a picker for the attribute displayed in the lines (word, lemma, tag …),
      attribute order, sizes and descriptions are taken from `/info/:corpusId`
- [x] lines with `errMsg` are shown as errors and are not dropped
- [ ] options form: contextWidth, contextStruct, noShuffle, subcorpus, showTextProps
- [ ] showing structures (`showMarkup=1`). Note: `TokenSlice.UnmarshalJSON`
      in mquery-common has no element type for `"close"` markup, so that has
      to be fixed in mquery-common first
- [ ] collocate highlighting (`coll`, `collRange`)
- [ ] maybe: a horizontal "interlinear" mode showing word/lemma/tag stacked

### M2 – token frequencies (`GET /freqs/:corpusId`) ✅ (first iteration)
- [x] table: value | freq | ipm | % of hits | bar (1/8-cell resolution)
- [x] `a` attribute picker (from `/info`; default: last used for the corpus → `lemma` → `word`)
- [x] `f` min. frequency (`flimit`) picker: presets or a typed number
- [x] `m` max. items picker (presets or a typed number up to 10000), `+`/`-` step through presets
- [x] `i` toggles `matchCase`, `s` sorts by freq/value
- [x] settings (attr per corpus, case, maxItems, flimit) are kept for the session
- [x] `enter` on a row → nested concordance (result views form a stack, `esc` returns).
      The query is built client-side (`cql.RestrictByValues`): each `[...]` position of the
      original query gets `& attr="value"` (with `(?i)` when case is ignored). Queries that
      are not a plain sequence of positions (`within`, `{n}`, `|`, …) fall back to value-only
      conditions; the concordance summary says so.
- [ ] word-list mode (regexp input) – there is no dedicated endpoint; maybe
      `/freqs` with `[word="<regexp>"]` and `attr=word` is good enough
- [ ] note: MQuery's `examplesQueryTpl` in the response is never filled – if it gets
      implemented server-side, it could replace the client-side query building

### M3 – text type frequencies (`GET /text-types/:corpusId`) ✅ (first iteration)
- [x] table: value | freq | text size (`base`) | ipm (relative to the text type size) | % of hits | bar
- [x] `s` sorts by freq / ipm / value; when sorted by ipm, the bar shows ipm.
      Note: the server returns the top `maxItems` *by freq*, so sorting by ipm only reorders those
- [x] `a` structural attribute picker combining
      - configured text properties (`/info?attachConf=1` → e.g. `author` = `bncdoc.author`)
      - all attributes from `/text-types-avail-values` (it skips attributes with too many values,
        e.g. authors/titles, hence the combination)
      - a typed custom `struct.attr`
      default: last used for the corpus → `text-type` property → first overview property → first item
- [x] `f` min freq, `m`/`+`/`-` max items (shared pickers with M2)
- [x] empty values are shown as `(empty)`
- [x] `enter` → nested concordance `(query) within <struct attr="value"/>`
      (counts verified to match the frequencies)
- [x] settings are remembered for the session only after a successful response
- [ ] overview mode (`/text-types-overview`: all overview attributes at once)
- [ ] `/text-types-norms` – not needed, `base` in the response already gives the text type size

### M4 – collocations (`GET /collocations/:corpusId`)
- options: `measure` (logDice, T-score, MI, …), `srchLeft`/`srchRight`,
  `srchAttr`, `minCollFreq`, `maxItems`
- table: collocate | score | freq
- `enter` → concordance filtered with `coll` (reuses M1)

### Shared UI components
- `selectList` (`ui/selectlist.go`): modal "pick one" box drawn over the view
  (`placeOverlay`), filter by typing (exact match first), current value marked `●`,
  optional hints per item, optional custom values with a validator
  (`withCustom(placeholder, parsePositiveInt(min, max))`). Intended for: text type
  attributes (M3), collocation measure/window/attr (M4)
- `listNav`: cursor + scroll for single-line-row lists
- `freqTable` (`ui/freqtable.go`): sortable frequency table with bars, optional `base` column;
  plus shared min freq / max items pickers (used by M2, M3)

### Later / cross-cutting
- config file (API URL, headers, defaults), query history
- subcorpus selection in the corpus picker (`/corplist` already returns them)
- export of the current result (TSV/JSON)
