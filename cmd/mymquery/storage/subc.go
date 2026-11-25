package storage

import (
	"time"
)

// Subcorpus is a saved subcorpus (identified by its ID only, see
// MQuery's /subcorpus endpoint) which has been used with a corpus
type Subcorpus struct {
	CorpusID string
	ID       string
	// Label is an optional user-defined name ("" = none)
	Label string
	// Size is the size in tokens as reported by the server when last used
	Size     int64
	LastUsed time.Time
}

// UseSubcorpus records that a subcorpus has been used with a corpus
// (creating it if needed). Its label is kept.
func (d *DB) UseSubcorpus(corpusID, subcID string, size int64) error {
	_, err := d.db.Exec(
		`INSERT INTO subcorpora (corpus_id, subc_id, size, last_used_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (corpus_id, subc_id) DO UPDATE SET
			size = excluded.size, last_used_at = excluded.last_used_at`,
		corpusID, subcID, size, time.Now().Unix(),
	)
	return err
}

// SetSubcorpusLabel sets a label of a stored subcorpus ("" removes it)
func (d *DB) SetSubcorpusLabel(corpusID, subcID, label string) error {
	_, err := d.db.Exec(
		"UPDATE subcorpora SET label = ? WHERE corpus_id = ? AND subc_id = ?",
		label, corpusID, subcID,
	)
	return err
}

// DeleteSubcorpus removes a subcorpus from the stored ones (the subcorpus
// itself is not affected, it is just forgotten)
func (d *DB) DeleteSubcorpus(corpusID, subcID string) error {
	_, err := d.db.Exec(
		"DELETE FROM subcorpora WHERE corpus_id = ? AND subc_id = ?", corpusID, subcID)
	return err
}

// Subcorpora returns subcorpora used with a corpus, the most recently used first
func (d *DB) Subcorpora(corpusID string) ([]Subcorpus, error) {
	rows, err := d.db.Query(
		`SELECT subc_id, label, size, last_used_at FROM subcorpora
		WHERE corpus_id = ? ORDER BY last_used_at DESC, subc_id`, corpusID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ans []Subcorpus
	for rows.Next() {
		item := Subcorpus{CorpusID: corpusID}
		var lastUsed int64
		if err := rows.Scan(&item.ID, &item.Label, &item.Size, &lastUsed); err != nil {
			return nil, err
		}
		item.LastUsed = time.Unix(lastUsed, 0)
		ans = append(ans, item)
	}
	return ans, rows.Err()
}
