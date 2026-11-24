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
query. The last query is shared among features with the same `QueryType`
(all current features take CQL), so e.g. a concordance query is prefilled
when switching to frequencies. Drill-down queries of nested views do not
change it.

## Milestones

### M0 – core shell ✅
- [x] menu with all functions (unimplemented ones are shown but disabled)
- [x] corpus selection from `/corplist` (filterable with `/`)
- [x] query input (CQL; prefilled with the last query, shared by features with the same `QueryType`)
- [x] query history (`ui/history.go`): submitted CQL queries, deduplicated, max. `-history-size`
      (default 100), stored in `-history-file` (default `$XDG_STATE_HOME/mymquery/history`,
      one query per line; empty = memory only). `↑/↓` (`ctrl+p/n`) browse it in the query input,
      going past the newest restores the typed draft. The last stored query prefills the input
      on startup
- [x] header showing function / corpus / query, contextual footer help
- [x] HTTP errors shown inline (MQuery's `{"error": ...}` is decoded)
- [x] `.env` support (`-env-file`, default `./.env`; `-no-env-file` skips it; real env vars take precedence):
      `API_URL` (or `MQUERY_API_URL`), `API_TOKEN`, `LOGIN_URL` (default `https://korpus.cz/login`).
      With `API_TOKEN` set, the app logs in on startup (POST `personal_access_token`,
      see https://wiki.korpus.cz/doku.php/en:manualy:api) and sends the session cookie
      with every request; failed login aborts startup. Header shows `api: <url>` on the
      right, plus `(authenticated)` when logged in

### M1 – concordance (`GET /concordance/:corpusId`) ✅ (first iteration)
- [x] KWIC layout: right-aligned left context, highlighted KWIC, right context
- [x] selectable line cursor, scrolling, paging (`rowsOffset`/`maxRows`)
- [x] line detail in a modal (`←/→` browse lines, `esc` closes):
      `enter` → all positional attributes of every token in a table,
      `t` → text properties and ref. The same key toggles the modal off,
      the other one switches between the two
- [x] `a` opens a picker for the attribute displayed in the lines (word, lemma, tag …),
      attribute order, sizes and descriptions are taken from `/info/:corpusId`
- [x] lines with `errMsg` are shown as errors and are not dropped
- [ ] options form: contextWidth, contextStruct, noShuffle, subcorpus, showTextProps
- [ ] showing structures (`showMarkup=1`). Note: `TokenSlice.UnmarshalJSON`
      in mquery-common has no element type for `"close"` markup, so that has
      to be fixed in mquery-common first
- [x] collocate filter + highlighting (`coll`, `collRange`) – used by the M4 drill-down
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

### M4 – collocations (`GET /collocations/:corpusId`) ✅ (first iteration)
- [x] table: collocate | score | freq | bar (score; freq when sorted by freq; negative scores = no bar)
- [x] `c` measure picker (all 9 MQuery measures with short descriptions, default logDice)
- [x] `a` attribute picker (`srchAttr`, default: last used → `lemma` → `word`)
- [x] `w` window picker (presets incl. left/right-only, or typed `L,R` / `N`, 0–50, not both 0)
- [x] `f` min. collocate freq (`minCollFreq`, default 3), `m`/`+`/`-` max items
- [x] `s` sorts by score / freq / value
- [x] `enter` → nested concordance filtered by `coll=[attr="value"]` and `collRange`
      (window `L,R` → `-L,R`; left-only → `-L,-1`; right-only → `1,R`). Counts verified to match
      collocate freqs; collocates are highlighted in the lines. The attr/window are taken from
      the response the table shows, not from (possibly failed) newer settings
- [ ] `minCorpFreq` (server uses `minCollFreq` by default)
- [ ] `/collocations-extended` (collocates with examples)

### Shared UI components
- `selectList` (`ui/selectlist.go`): modal "pick one" box drawn over the view
  (`placeOverlay`), filter by typing (exact match first), current value marked `●`,
  optional hints per item, optional custom values with a validator
  (`withCustom(placeholder, parsePositiveInt(min, max))`). Intended for: text type
  attributes (M3), collocation measure/window/attr (M4)
- `modal` (`ui/modal.go`): scrollable bordered window over a dimmed view, with a title,
  subtitle and scroll position. The box fits the content (grow-only while open so it
  does not jump when the content changes). The owner handles open/close and its own keys,
  the modal handles scrolling (`↑/↓`, `pgup/pgdn`, `g/G`). Used by the M1 line detail
- `csvDialog` (`ui/savecsv.go`): modal path input which writes prepared rows as CSV
  (`~` expansion, overwrite confirmation)
- `listNav`: cursor + scroll for single-line-row lists
- `freqTable` (`ui/freqtable.go`): sortable frequency table with bars, optional `base` column;
  plus shared min freq / max items pickers (used by M2, M3)

### Later / cross-cutting
- config file (API URL, headers, defaults)
- query history: a searchable picker (`ctrl+r`, `selectList`), maybe remember corpus per entry
- subcorpus selection in the corpus picker (`/corplist` already returns them)
- [x] `e` saves the token freqs / text types / collocations table as CSV (`ui/savecsv.go`):
      a path dialog prefilled with `<token-freqs|text-types|colls>-<hash>.csv`, where the hash
      covers the corpus and all request args (query, attr, limits, …) of the shown result.
      Rows are in the current sort order, raw numbers; existing files need a second `enter`
- export of the concordance (TSV/JSON)
- select list: rank prefix matches above "contains" matches (typing `le` currently
  offers `lempos` before `lemma` because of the corpus attribute order)
- text types: sorting by ipm reorders only the top `maxItems` by freq (server-side limit);
  maybe hint this in the UI or fetch more items when sorting by ipm
- build: `mymquery` is not part of the generated Makefile (`./configure`); it is pure Go,
  so `go build -o mymquery ./cmd/mymquery` works. Consider adding the binary to `.gitignore`

### Known server-side / library issues (found while building the TUI)
- mquery-common line parser: lines containing multi-word tokens (e.g. BNC `away from`)
  come back with `errMsg: unparseable Manatee KWIC line: expected 4N elms…`
  (the parser splits on whitespace). The TUI shows them as ⚠ lines
- mquery-common `TokenSlice.UnmarshalJSON` has no element type for `"close"` markup
  → `showMarkup=1` will likely fail to decode (blocks M1 "showing structures")
- `/text-types` on KSP fails with `FileAccessError (…/KSP/doc.date.token)` – missing
  struct attr `.token` coverage file (see the mktokencov follow-up)
- `/freqs` response field `examplesQueryTpl` is never filled
- `/collocations` with `minItems` > number of results returns an empty body (handler just returns)

### Testing notes
- unit tests: `go test ./cmd/mymquery/...` (CQL helpers, select list, collocation window)
- manual: run in a detached tmux session and use `capture-pane -p` (`-e` for colours);
  send keys one by one (a burst like `/bnc` is parsed as one multi-rune key)
