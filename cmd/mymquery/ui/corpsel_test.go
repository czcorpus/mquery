package ui

import (
	"testing"

	"mquery/cmd/mymquery/api"

	"github.com/stretchr/testify/assert"
)

func corpusIDs(cc []api.CorpusItem) []string {
	ans := make([]string, len(cc))
	for i, c := range cc {
		ans[i] = c.ID
	}
	return ans
}

func TestFilterCorpora(t *testing.T) {
	corpora := []api.CorpusItem{
		{ID: "syn2020", FullName: "SYN2020", Language: "cs"},
		{ID: "intercorp_en", FullName: "InterCorp English", Language: "en"},
		{ID: "syn", FullName: "SYN", Language: "cs"},
		{ID: "oral", FullName: "ORAL v1", Language: "cs"},
	}
	assert.Equal(t, []string{"syn2020", "intercorp_en", "syn", "oral"}, corpusIDs(filterCorpora(corpora, "")))
	assert.Equal(t, []string{"syn", "syn2020"}, corpusIDs(filterCorpora(corpora, "SyN")))
	assert.Equal(t, []string{"intercorp_en"}, corpusIDs(filterCorpora(corpora, "corp eng")))
	assert.Equal(t, []string{"intercorp_en"}, corpusIDs(filterCorpora(corpora, "en")))
	assert.Empty(t, filterCorpora(corpora, "xyz"))
}

func TestFilterCorporaWordsAnyOrder(t *testing.T) {
	corpora := []api.CorpusItem{
		{ID: "intercorp_v17ud_cs", FullName: "InterCorp v17 UD Czech", Language: "cs"},
		{ID: "intercorp_v17ud_en", FullName: "InterCorp v17 UD English", Language: "en"},
		{ID: "intercorp_v16ud_cs", FullName: "InterCorp v16 UD Czech", Language: "cs"},
		{ID: "syn2020", FullName: "SYN2020", Language: "cs"},
	}
	assert.Equal(t, []string{"intercorp_v17ud_cs"}, corpusIDs(filterCorpora(corpora, "intercorp cs 17")))
	assert.Equal(t, []string{"intercorp_v17ud_cs"}, corpusIDs(filterCorpora(corpora, "  CS   17  InterCorp ")))
	assert.Equal(t,
		[]string{"intercorp_v17ud_cs", "intercorp_v16ud_cs"},
		corpusIDs(filterCorpora(corpora, "czech intercorp")))
	assert.Empty(t, filterCorpora(corpora, "intercorp 17 xyz"))
	// exact match of the whole query still goes first
	assert.Equal(t, []string{"syn2020"}, corpusIDs(filterCorpora(corpora, "syn2020")))
}
