package storage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subcIDs(items []Subcorpus) []string {
	ans := make([]string, len(items))
	for i, it := range items {
		ans[i] = it.ID
	}
	return ans
}

func TestSubcorpora(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "test.db"))
	items, err := db.Subcorpora("syn")
	require.NoError(t, err)
	assert.Empty(t, items)

	require.NoError(t, db.UseSubcorpus("syn", "ab12", 100))
	require.NoError(t, db.UseSubcorpus("syn", "cd34", 200))
	require.NoError(t, db.UseSubcorpus("bnc", "ef56", 300))
	require.NoError(t, db.SetSubcorpusLabel("syn", "ab12", "fiction"))
	// using it again updates the size but keeps the label
	require.NoError(t, db.UseSubcorpus("syn", "ab12", 150))

	items, err = db.Subcorpora("syn")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.ElementsMatch(t, []string{"ab12", "cd34"}, subcIDs(items))
	for _, it := range items {
		if it.ID == "ab12" {
			assert.Equal(t, "fiction", it.Label)
			assert.Equal(t, int64(150), it.Size)
			assert.False(t, it.LastUsed.IsZero())
		}
	}

	require.NoError(t, db.DeleteSubcorpus("syn", "cd34"))
	items, err = db.Subcorpora("syn")
	require.NoError(t, err)
	assert.Equal(t, []string{"ab12"}, subcIDs(items))

	items, err = db.Subcorpora("bnc")
	require.NoError(t, err)
	assert.Equal(t, []string{"ef56"}, subcIDs(items))
}

func TestAddQueryWithSubcorpus(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, db.AddQuery("syn", "ab12", "conc", "x"))
	var subc string
	require.NoError(t, db.db.QueryRow("SELECT subcorpus FROM queries").Scan(&subc))
	assert.Equal(t, "ab12", subc)
}
