package ui

import (
	"testing"
	"time"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEnv(t *testing.T) *Env {
	// nothing listens here - any unexpected data request fails
	client, err := api.NewClient("http://127.0.0.1:1", nil, time.Second)
	require.NoError(t, err)
	return NewEnv(client, "", 50, 10)
}

func TestChatResultFreqsPreloaded(t *testing.T) {
	env := newTestEnv(t)
	store := agent.NewStore()
	res := store.Add("freqs", map[string]any{
		"corpus_id": "syn", "q": `[lemma="pes"]`, "attr": "word", "max_items": float64(5),
	}, `{"concSize":3,"freqs":[{"word":"psa","freq":2},{"word":"pes","freq":1}]}`, true)

	view := newChatResultView(env, res)
	fv, ok := view.(*freqsView)
	require.True(t, ok)
	assert.Equal(t, "syn", fv.CorpusID())
	assert.Equal(t, 5, fv.maxItems)
	assert.Equal(t, mcpDefaultFLimit, fv.flimit)

	// attributes are loaded first; then the preloaded response is used
	fv.Init()
	_, cmd := fv.Update(corpusAttrsMsg{
		viewID: fv.id, corpusID: "syn",
		attrs: []corpusAttr{{Name: "word"}, {Name: "lemma"}},
	})
	msg, ok := cmd().(freqsLoadedMsg)
	require.True(t, ok)
	assert.True(t, msg.preloaded)
	assert.Equal(t, "word", msg.args.Attr)
	require.NotNil(t, msg.resp)
	assert.Len(t, msg.resp.Freqs, 2)

	fv.Update(msg)
	assert.Equal(t, "word", fv.attr)
	assert.False(t, fv.loading)
	// settings chosen by the model are not remembered as the user's ones
	assert.Empty(t, env.freqAttr["syn"])
	assert.Equal(t, freqMaxItemsSteps[0], env.freqMaxItems)
}

func TestChatResultCollsArgs(t *testing.T) {
	env := newTestEnv(t)
	res := agent.NewStore().Add("collocations", map[string]any{
		"corpus_id": "syn", "q": "x", "srch_left": float64(2), "srch_right": float64(0),
		"measure": "tScore",
	}, `{"concSize":3,"colls":[{"word":"a","score":1.5,"freq":2}]}`, true)
	cv, ok := newChatResultView(env, res).(*collsView)
	require.True(t, ok)
	assert.Equal(t, [2]int{2, 0}, cv.window)
	assert.Equal(t, "tScore", cv.measure)
	assert.Equal(t, mcpDefaultCollAttr, cv.attr)
	assert.Equal(t, mcpDefaultMinCollFreq, cv.minFreq)
	require.NotNil(t, cv.preloaded)
}

func TestChatResultConcPreloaded(t *testing.T) {
	env := newTestEnv(t)
	res := agent.NewStore().Add("concordance", map[string]any{
		"corpus_id": "syn", "q": "x", "rows_offset": float64(20),
		"coll": `[lemma="y"]`, "coll_range": "-3,2",
	}, `{"concSize":100,"lines":[]}`, true)
	cv, ok := newChatResultView(env, res).(*concView)
	require.True(t, ok)
	assert.Equal(t, 20, cv.rowsOffset)
	assert.Equal(t, `[lemma="y"]`, cv.coll)
	assert.Equal(t, [2]int{-3, 2}, cv.collRange)
	msg, ok := cv.fetch()().(concLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, 100, msg.resp.ConcSize)
}

func TestChatResultRawFallback(t *testing.T) {
	env := newTestEnv(t)
	store := agent.NewStore()
	cases := []*agent.Result{
		store.Add("concordance", map[string]any{"corpus_id": "syn", "q": "x", "format": "markdown"}, "| a |", true),
		store.Add("freqs", map[string]any{"corpus_id": "syn", "q": "x"}, "error: invalid query", false),
		store.Add("freqs", map[string]any{"corpus_id": "syn", "q": "x", "subcorpus": "s1"}, `{"freqs":[]}`, true),
		store.Add("corpus_info", map[string]any{"corpus_id": "syn"}, `{"corpus":{}}`, true),
		store.Add("freqs", map[string]any{"corpus_id": "syn", "q": "x"}, `not a JSON`, true),
	}
	for _, res := range cases {
		_, ok := newChatResultView(env, res).(*rawResultView)
		assert.True(t, ok, res.ID)
	}
}

func TestFormatToolArgs(t *testing.T) {
	assert.Equal(t,
		`corpus_id=syn q=[word="a"] attr=lemma match_case=true max_items=10`,
		formatToolArgs(map[string]any{
			"max_items": float64(10), "q": `[word="a"]`, "attr": "lemma",
			"corpus_id": "syn", "match_case": true,
		}))
}
