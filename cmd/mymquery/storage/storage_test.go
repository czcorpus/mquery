package storage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T, path string) *DB {
	db, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRecentQueries(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "sub", "test.db"))
	qs, err := db.RecentQueries(10)
	require.NoError(t, err)
	assert.Empty(t, qs)

	for _, q := range []string{"a", "b", "a", "c", "d"} {
		require.NoError(t, db.AddQuery("syn", "conc", q))
	}
	qs, err = db.RecentQueries(10)
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a", "c", "d"}, qs)

	qs, err = db.RecentQueries(2)
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "d"}, qs)
}

func TestReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db := openTestDB(t, path)
	require.NoError(t, db.AddQuery("syn", "freqs", "x"))
	require.NoError(t, db.Close())

	db2 := openTestDB(t, path) // migrations are not applied again
	qs, err := db2.RecentQueries(10)
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, qs)
}
