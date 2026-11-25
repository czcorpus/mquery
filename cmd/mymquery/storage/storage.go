// Package storage is a local SQLite database of mymquery
// (query history, AI chat conversations).
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultPath returns $XDG_STATE_HOME/mymquery/mymquery.db
// (~/.local/state/mymquery/mymquery.db when XDG_STATE_HOME is not set)
func DefaultPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "mymquery", "mymquery.db")
}

// migrations are applied in order; the number of applied ones
// is stored as PRAGMA user_version (never change an existing item!)
var migrations = []string{
	`CREATE TABLE queries (
		id INTEGER PRIMARY KEY,
		created_at INTEGER NOT NULL,
		corpus_id TEXT NOT NULL,
		feature TEXT NOT NULL,
		query TEXT NOT NULL
	);
	CREATE INDEX queries_query ON queries(query);`,

	`CREATE TABLE chats (
		id INTEGER PRIMARY KEY,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		title TEXT NOT NULL,
		vendor TEXT NOT NULL,
		model TEXT NOT NULL,
		corpus_id TEXT NOT NULL,
		input_tokens INTEGER NOT NULL,
		cached_input_tokens INTEGER NOT NULL,
		output_tokens INTEGER NOT NULL,
		reasoning_tokens INTEGER NOT NULL
	);
	CREATE TABLE chat_messages (
		chat_id INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		seq INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT,
		tool_result TEXT,
		opaque BLOB,
		PRIMARY KEY (chat_id, seq)
	);
	CREATE TABLE chat_results (
		chat_id INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		seq INTEGER NOT NULL,
		result_id TEXT NOT NULL,
		tool TEXT NOT NULL,
		args TEXT,
		content TEXT NOT NULL,
		ok INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		PRIMARY KEY (chat_id, seq)
	);`,

	`ALTER TABLE queries ADD COLUMN subcorpus TEXT NOT NULL DEFAULT '';
	CREATE TABLE subcorpora (
		corpus_id TEXT NOT NULL,
		subc_id TEXT NOT NULL,
		label TEXT NOT NULL DEFAULT '',
		size INTEGER NOT NULL DEFAULT 0,
		last_used_at INTEGER NOT NULL,
		PRIMARY KEY (corpus_id, subc_id)
	);`,
}

type DB struct {
	db *sql.DB
}

// Open opens (and creates or upgrades if needed) the database
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// busy_timeout lets more running instances of the app share the file
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// writes are serialized anyway; a single connection also guarantees
	// the per-connection pragmas apply
	db.SetMaxOpenConns(1)
	ans := &DB{db: db}
	if err := ans.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database %s: %w", path, err)
	}
	return ans, nil
}

func (d *DB) migrate() error {
	var version int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("database version %d is newer than supported %d", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		tx, err := d.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		// PRAGMA does not support placeholders
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

// AddQuery logs a submitted query (subcorpus "" = the whole corpus)
func (d *DB) AddQuery(corpusID, subcorpus, feature, query string) error {
	_, err := d.db.Exec(
		"INSERT INTO queries (created_at, corpus_id, subcorpus, feature, query) VALUES (?, ?, ?, ?, ?)",
		time.Now().Unix(), corpusID, subcorpus, feature, query,
	)
	return err
}

// RecentQueries returns up to `limit` distinct queries, the most recently
// used one last
func (d *DB) RecentQueries(limit int) ([]string, error) {
	rows, err := d.db.Query(
		`SELECT query FROM (
			SELECT query, MAX(id) AS last_id FROM queries
			GROUP BY query ORDER BY last_id DESC LIMIT ?
		) ORDER BY last_id`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ans []string
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			return nil, err
		}
		ans = append(ans, q)
	}
	return ans, rows.Err()
}
