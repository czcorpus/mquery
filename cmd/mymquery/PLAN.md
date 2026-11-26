# mymquery – terminal UI for MQuery

A Bubble Tea based TUI client for the MQuery HTTP API. It is a pure HTTP
client: it uses only the public GET endpoints, no CGO and no `mquery/*`
server packages (the only exception is the CGO-free `mquery/mcp` package,
embedded for the AI chat). Response types are taken from `mquery-common`
where possible.

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
    subc.go        subcorpus picker (ctrl+s in the query input)
    concopts.go    concordance options (form)
    form.go        optionsForm (a reusable modal form)
    conc.go        concordance result view
    chat.go        AI chat screen (M5)
    chatresult.go  stored chat results → result views (or a raw view)
    chatlist.go    saved chats list (open / delete)
    markdown.go    markdown rendering of chat answers (glamour + cache)
    mdtable.go     markdown tables (rendered by us at their natural width)
  llm/             provider-neutral chat completion + tool calling (ported from NLQL)
    genai.go       Gemini backend
    mistral.go     Mistral backend (plain HTTP + SSE, ported from NLQL)
    models.go      primary + fallback models (shared by the backends)
  agent/           AI chat: in-process MQuery MCP server, agent loop, result store
  storage/         local SQLite DB: query history, saved chats
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
      (default 100), stored in the local DB (see below; `-db ""` = memory only). `↑/↓` (`ctrl+p/n`) browse it in the query input,
      going past the newest restores the typed draft. The last stored query prefills the input
      on startup
- [x] header showing function / corpus / query, contextual footer help
- [x] local SQLite database (`storage/`, pure Go `modernc.org/sqlite`, pinned to v1.59 as newer
      versions require go 1.26): `-db` (default `$XDG_STATE_HOME/mymquery/mymquery.db`), WAL +
      busy timeout so more instances can share it, migrations tracked by `PRAGMA user_version`.
      Table `queries` logs every submitted query with time, corpus and feature (`Feature.ID`);
      the history shows distinct queries ordered by their last use
- [x] subcorpus selection (`ui/subc.go`, `ctrl+s` in the query input). Only IDs are used: the CNC
      information system with subcorpus metadata is not accessible, and MQuery resolves an ID
      itself (configured alias first, then `<savedSubcorporaDir>/<ID[:2]>/<ID>/data.subc`). The picker
      offers the whole corpus, configured aliases (from `/corplist`), saved subcorpora used before
      (local DB table `subcorpora`: size, last use, optional label via `ctrl+r`, `ctrl+x` forgets one)
      and a typed ID. Saved ones are verified via MQuery's `/subcorpus/:corpusId/:subcorpusId` before
      they are accepted (unknown → 404; a `.subc` spanning beyond the corpus end → 422 – the file has
      no reference to its corpus, so this is the only possible check). The choice is session state
      (`searchTarget` = corpus + subcorpus): shown in the header (`syn2020 / ID`) and on the query
      screen (size, % of the corpus), sent with all requests, inherited by drill-downs, included in
      CSV file name hashes and logged in `queries.subcorpus`. Selecting a different corpus resets it.
      The AI chat gets it in the system prompt
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
- [x] options form (`o`, `ui/concopts.go`): context width (0–50), context limited to a structure
      (corpus default = MQuery's `viewContextStruct`, typically a sentence; "tokens only" sends an
      empty `contextStruct` to override it; or any structure from `/info`), line order
      (shuffled / corpus order = `noShuffle`), text properties (none/basic/all), lines per page.
      Settings are session-wide (also for drill-downs; `-context-width` / `-page-size` are the
      defaults). Changing the order resets paging, a page size change keeps the current first line's
      page. Non-default context/order are shown in the summary line
- [x] showing structures (`showMarkup=1`, "Structures" in the options form; which structures are
      sent is decided by the corpus' `ConcMarkupStructures`). Tags are rendered muted between
      tokens (`<s>`, `</p>`, `<lb/>`), the glue structure `<g/>` joins the surrounding tokens
      instead (also around KWIC, keeping its column). Needed mquery-common v0.10.6
      (decoding of `"close"` and self-closing markup)
- [x] collocate filter + highlighting (`coll`, `collRange`) – used by the M4 drill-down
- [x] collocate filter in the query input: the concordance query screen has extra fields
      "Collocate (CQL)" and "Collocate window" (`left,right`, shared with the collocations
      window setting); `tab`/`shift+tab` switch fields, an empty collocate = no filter.
      Changing the corpus from the query screen moved from `tab` to `ctrl+o`
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

### M5 – AI chat ✅ (first iteration)
A chat with an LLM which queries MQuery via an in-process MQuery MCP server
(`mquery/mcp`, sharing the HTTP client – incl. the session cookie – with the UI).
- [x] `llm` package: `Backend` interface (non-streaming), Gemini backend (`google.golang.org/genai`)
      with fallback models and retries on 429/5xx. Config: `GEMINI_API_KEY`, `LLM_MODEL` /
      `-llm-model` (comma separated, first = primary; default `gemini-3.5-flash-lite,gemini-3.1-flash-lite`)
- [x] `agent` package: agent loop (max 15 iterations), builtin `corpus_list` tool (MCP has none),
      session `Store` of all tool results (`r1`, `r2`, …; failed calls too). The model gets
      results prefixed with `result_id: rN`, truncated to 40 kB; it is asked to cite `[rN]`.
      A failed/cancelled turn keeps its progress up to the last completed step (user message or
      a complete set of tool results): `ctrl+r` continues from there (tool calls are not repeated,
      new iteration budget), a new message discards the failed turn
- [x] chat screen (menu item 5): transcript (viewport) + textarea; `enter` send, `alt+enter`
      newline, `esc` cancels a running turn, `ctrl+n` new conversation (clears the screen,
      new result store), `tab` browses results
      (`↑/↓`, `enter` opens, `esc` returns to the chat with the selection kept)
- [x] results open in the regular views with the model's settings and preloaded data (no
      requery): concordance, token freqs, text types, collocations; anything else (or failed /
      markdown / subcorpus results) in a raw (pretty JSON) view. Settings chosen by the model are
      not remembered as the user's session settings
- [x] header: model name and session token usage
- [x] Mistral backend (`llm/mistral.go`, ported from NLQL, plus streaming): `LLM_BACKEND` / `-llm-backend`
      = `gemini` | `mistral` (empty = the first one with an API key: `GEMINI_API_KEY`, `MISTRAL_API_KEY`).
      `LLM_MODEL` empty = per-backend defaults (Mistral: `mistral-medium-latest,mistral-small-latest`).
      Fallback models on 429/5xx work the same as for Gemini (`llm/models.go`). Reasoning models return
      content as chunk arrays; they are replayed as `Opaque` (rebuilt from the parts when streaming)
- [ ] more backends: OpenAI-compatible (llama.cpp), Anthropic (nlql has them); they can
      implement just `llm.Backend` first and `StreamCompletion` later
- [x] streaming of answers: optional `llm.StreamingBackend` (Gemini: `GenerateContentStream`; request
      building and response parsing are shared with the non-streaming call). The callback gets the
      accumulated text, a retried attempt starts with `""` (= discard the partial text). The agent
      emits `TextStreaming` events (`Session.Stream`, on by default); the chat shows the text below the
      transcript ("writing…") until the completion finishes – then it becomes the final answer or an
      interim text (if tool calls follow); a partial text of a failed turn is not kept. Note: Gemini can
      fail with 503 in the middle of a stream – this is retried like any other 503
- [ ] a list of all stored results (also from previous conversations) + jump to the chat from anywhere
- [x] subcorpus support in result views (chat results with `subcorpus` open in their dedicated view)
- [x] chats are saved to the local DB (tables `chats`, `chat_messages`, `chat_results`) via
      `agent.Recorder` (implemented by `storage.DB`) after each completed step: the user message,
      each complete set of tool results, the final answer, a failed turn (up to its checkpoint).
      Saving is incremental (`ChatUpdate.KeepMessages` + new messages/results); a save failure is
      shown once per turn and does not fail it. `llm.Message/ToolCall.Opaque` became `[]byte`
      (Gemini thought signatures are saved). `Session.Load` restores a chat: an incomplete last turn
      (failed, interrupted by quitting) can be retried, result IDs continue the numbering, opaque
      state is dropped if the chat was run by another vendor
- [x] saved chats list (`ctrl+o` in the chat input; replaces the transcript): newest first, the open
      chat marked `●` and preselected; `enter` opens a chat (the transcript is rebuilt from saved
      messages – tool messages are mapped to results via their `result_id:` prefix; an incomplete
      last turn offers `ctrl+r`), `d` + `y` deletes (deleting the open chat starts a new one).
      Results of a reopened chat open in their views without requerying
- [ ] saved chats: filtering/search, renaming, opening the list right from the menu
- [ ] note: `chats.updated_at` has a 1 s resolution (fine for sorting in practice)
- [x] markdown rendering of answers (`ui/markdown.go`): glamour with its dark/light style adjusted (no
      margins, default text color, accent headings). Rendered answers are cached per width (the
      transcript is re-rendered on every spinner tick); a streamed answer is rendered on each update
      (a partial markup may look odd for a moment). `[rN]` references are replaced by private-use
      runes before rendering and highlighted afterwards. Tables are split out and rendered by us
      (`ui/mdtable.go`) – glamour always stretches them to the full width. Interim texts stay plain.
      The system prompt now asks for markdown (lists, bold, small tables)
- [ ] clickable `[rN]` references
- [ ] the MCP server sends `X-Mquery-Public-Client: 1`, so the chat may see fewer corpora than
      the regular UI when running on an internal network
- [ ] token costs: `corpus_list` and `corpus_info` outputs are large and re-sent each iteration

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
- `optionsForm` (`ui/form.go`): modal form with choice fields (`←/→`) and validated input fields;
  `↑/↓`/`tab` move, `enter` validates and applies. Used by the concordance options; the per-option
  pickers of M2–M4 could move to it
- `csvDialog` (`ui/savecsv.go`): modal path input which writes prepared rows as CSV
  (`~` expansion, overwrite confirmation)
- `listNav`: cursor + scroll for single-line-row lists
- `freqTable` (`ui/freqtable.go`): sortable frequency table with bars, optional `base` column;
  plus shared min freq / max items pickers (used by M2, M3)

### Later / cross-cutting
- config file (API URL, headers, defaults)
- query history: a searchable picker (`ctrl+r`, `selectList`), maybe filter by corpus (already stored)
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
- ~~mquery-common line parser: multi-word tokens (e.g. BNC `away from`) gave `errMsg`~~
  – fixed in mquery-common v0.10.5 (no errors in 200 BNC `away` lines)
- ~~mquery-common `TokenSlice.UnmarshalJSON` could not decode `"close"` markup~~ – fixed in v0.10.6
- `/concordance` shuffling is random per request (`conc->shuffle()` in `mango.cc` on every
  call), so paging through a shuffled concordance is not stable – pages may repeat or skip
  lines, and changing e.g. "Structures" shows different lines. Would need a fixed seed
  (e.g. derived from the query) or a cached shuffled concordance
- `/text-types` on KSP fails with `FileAccessError (…/KSP/doc.date.token)` – missing
  struct attr `.token` coverage file (see the mktokencov follow-up)
- `/freqs` response field `examplesQueryTpl` is never filled
- `/collocations` with `minItems` > number of results returns an empty body (handler just returns)
- ~~`/concordance` and `/term-frequency` in a subcorpus: `ipm` relative to the whole corpus~~
  – fixed in the server (PR #292): ipm is relative to the subcorpus, responses contain `subcSize`
  (`corpusSize` stays the whole corpus); `/freqs` now fills its `subcSize` too
- `/collocations` returns `subcSize` equal to the corpus size even without a subcorpus
  (other endpoints omit it); left as is to keep the response unchanged

### Testing notes
- unit tests: `go test ./cmd/mymquery/...` (CQL helpers, select list, collocation window)
- manual: run in a detached tmux session and use `capture-pane -p` (`-e` for colours);
  send keys one by one (a burst like `/bnc` is parsed as one multi-rune key)
