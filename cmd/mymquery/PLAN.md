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
- [x] `a` switches the attribute displayed in the lines (word → lemma → tag …),
      attribute order is taken from `/info/:corpusId`
- [x] lines with `errMsg` are shown as errors and are not dropped
- [ ] options form: contextWidth, contextStruct, noShuffle, subcorpus, showTextProps
- [ ] showing structures (`showMarkup=1`). Note: `TokenSlice.UnmarshalJSON`
      in mquery-common has no element type for `"close"` markup, so that has
      to be fixed in mquery-common first
- [ ] collocate highlighting (`coll`, `collRange`)
- [ ] maybe: a horizontal "interlinear" mode showing word/lemma/tag stacked

### M2 – token frequencies (`GET /freqs/:corpusId`)
- query = CQL; options: `attr` (default lemma), `matchCase`, `maxItems`, `flimit`
- table view: value | freq | ipm (sortable by column), plus a simple inline bar
- `enter` on a row → open a concordance for that value (reuses M1)
- word-list mode (regexp input): check which endpoint fits best,
  as there is no dedicated word-list endpoint yet

### M3 – text type frequencies (`GET /text-types/:corpusId`)
- options: `attr` chosen from `/text-types-overview` or the corpus's text properties
- table: value | freq | ipm | norm (from `/text-types-norms` when available)
- maybe: `/text-types-avail-values` for drilling into a struct attribute

### M4 – collocations (`GET /collocations/:corpusId`)
- options: `measure` (logDice, T-score, MI, …), `srchLeft`/`srchRight`,
  `srchAttr`, `minCollFreq`, `maxItems`
- table: collocate | score | freq
- `enter` → concordance filtered with `coll` (reuses M1)

### Later / cross-cutting
- config file (API URL, headers, defaults), query history
- subcorpus selection in the corpus picker (`/corplist` already returns them)
- export of the current result (TSV/JSON)
