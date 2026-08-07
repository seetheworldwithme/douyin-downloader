package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO
)

// Database wraps a SQLite connection used for download history, aweme records,
// transcript jobs, and task-center job persistence.
//
// Ported from storage/database.py. The Python original lazily opens a single
// aiosqlite connection behind an asyncio.Lock; here we open a single
// *sql.DB connection pool with max open / idle = 1 so all serialized writes go
// through one connection (SQLite's WAL mode allows concurrent readers, but
// modernc.org/sqlite is simplest to reason about when writes serialize).
type Database struct {
	dbPath      string
	db          *sql.DB
	initialized bool
}

// NewDatabase constructs a Database handle. It does NOT open the connection —
// call Initialize() once before first use.
func NewDatabase(dbPath string) *Database {
	if dbPath == "" {
		dbPath = "dy_downloader.db"
	}
	return &Database{dbPath: dbPath}
}

// Initialize opens the SQLite database, applies WAL + NORMAL synchronous, and
// creates all tables / indexes if they don't already exist. Safe to call
// multiple times — second and subsequent calls are no-ops.
func (d *Database) Initialize() error {
	if d.initialized {
		return nil
	}

	db, err := sql.Open("sqlite", d.dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite %s: %w", d.dbPath, err)
	}
	// Single connection so PRAGMA and writes serialize predictably.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return fmt.Errorf("set WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return fmt.Errorf("set synchronous=NORMAL: %w", err)
	}

	if err := d.createTables(db); err != nil {
		db.Close()
		return err
	}

	d.db = db
	d.initialized = true
	return nil
}

func (d *Database) createTables(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS aweme (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			aweme_id TEXT UNIQUE NOT NULL,
			aweme_type TEXT NOT NULL,
			title TEXT,
			author_id TEXT,
			author_name TEXT,
			author_sec_uid TEXT,
			create_time INTEGER,
			download_time INTEGER,
			file_path TEXT,
			metadata TEXT,
			cover_urls TEXT NOT NULL DEFAULT '',
			job_id TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS download_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			url_type TEXT NOT NULL,
			download_time INTEGER,
			total_count INTEGER,
			success_count INTEGER,
			config TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS transcript_job (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			aweme_id TEXT NOT NULL,
			video_path TEXT NOT NULL,
			transcript_dir TEXT,
			text_path TEXT,
			json_path TEXT,
			model TEXT NOT NULL,
			status TEXT NOT NULL,
			skip_reason TEXT,
			error_message TEXT,
			created_at INTEGER,
			updated_at INTEGER,
			UNIQUE(aweme_id, video_path, model)
		)`,
		`CREATE TABLE IF NOT EXISTS job (
			job_id              TEXT PRIMARY KEY,
			url                 TEXT NOT NULL,
			status              TEXT NOT NULL,
			created_at          TEXT NOT NULL,
			started_at          TEXT,
			finished_at         TEXT,
			total               INTEGER NOT NULL DEFAULT 0,
			success             INTEGER NOT NULL DEFAULT 0,
			failed              INTEGER NOT NULL DEFAULT 0,
			skipped             INTEGER NOT NULL DEFAULT 0,
			error               TEXT,
			author_nickname     TEXT,
			author_sec_uid      TEXT,
			retry_count         INTEGER NOT NULL DEFAULT 0,
			last_retry_at       TEXT,
			last_retry_summary  TEXT,
			retry_history       TEXT,
			overrides           TEXT
		)`,
	}
	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_aweme_id ON aweme(aweme_id)",
		"CREATE INDEX IF NOT EXISTS idx_author_id ON aweme(author_id)",
		"CREATE INDEX IF NOT EXISTS idx_download_time ON aweme(download_time)",
		"CREATE INDEX IF NOT EXISTS idx_transcript_aweme_id ON transcript_job(aweme_id)",
		"CREATE INDEX IF NOT EXISTS idx_transcript_status ON transcript_job(status)",
		"CREATE INDEX IF NOT EXISTS idx_job_created_at ON job(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_job_status ON job(status)",
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}
	return nil
}

// DB exposes the underlying *sql.DB for callers that need direct access.
// Initialize() must have been called.
func (d *Database) DB() *sql.DB {
	return d.db
}

// Close closes the underlying connection and allows Initialize() to be called
// again (matching the Python reset semantics).
func (d *Database) Close() error {
	if d.db == nil {
		return nil
	}
	err := d.db.Close()
	d.db = nil
	d.initialized = false
	return err
}

// ---------------- aweme queries ----------------

// IsDownloaded reports whether a row exists for awemeID with a non-empty
// file_path. Rows synced from the desktop sibling can exist with empty
// file_path and must NOT be treated as downloaded (see database.py comment).
func (d *Database) IsDownloaded(awemeID string) (bool, error) {
	var id int64
	err := d.db.QueryRow(
		"SELECT id FROM aweme WHERE aweme_id = ? AND file_path IS NOT NULL AND file_path != ''",
		awemeID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// AwemeRecord carries the fields persisted to the aweme table.
type AwemeRecord struct {
	AwemeID      string
	AwemeType    string
	Title        string
	AuthorID     string
	AuthorName   string
	AuthorSecUID string
	CreateTime   sql.NullInt64
	FilePath     string
	Metadata     string
	CoverURLs   string // JSON array string, may be ""
	JobID       string
}

// RecordDownload upserts an aweme row. Fields are preserved on conflict:
// empty incoming values never clobber existing non-empty values (matching the
// Python upsert semantics, so desktop-sibling my-content syncs don't blow
// away downloaded artifacts).
func (d *Database) RecordDownload(rec AwemeRecord) error {
	now := time.Now().Unix()
	var downloadTime any
	if rec.FilePath != "" {
		downloadTime = now
	}
	_, err := d.db.Exec(awemeUpsertSQL,
		nullableStr(rec.AwemeID),
		nullableStr(rec.AwemeType),
		nullableStr(rec.Title),
		nullableStr(rec.AuthorID),
		nullableStr(rec.AuthorName),
		nullableStr(rec.AuthorSecUID),
		nullableInt(rec.CreateTime),
		downloadTime,
		nullableStr(rec.FilePath),
		nullableStr(rec.Metadata),
		rec.CoverURLs,
		rec.JobID,
	)
	return err
}

// GetLatestAwemeTime returns the maximum create_time among downloaded rows for
// the given author sec UID, or 0 if none. Non-downloaded rows (empty
// file_path) are excluded so they don't poison the incremental baseline.
func (d *Database) GetLatestAwemeTime(secUID string) (int64, error) {
	var maxTime sql.NullInt64
	err := d.db.QueryRow(
		"SELECT MAX(create_time) FROM aweme WHERE author_id = ? "+
			"AND file_path IS NOT NULL AND file_path != ''",
		secUID,
	).Scan(&maxTime)
	if err != nil {
		return 0, err
	}
	if !maxTime.Valid {
		return 0, nil
	}
	return maxTime.Int64, nil
}

// AddHistory records a download_history row.
type HistoryRecord struct {
	URL          string
	URLType      string
	TotalCount   sql.NullInt64
	SuccessCount sql.NullInt64
	Config       string
}

func (d *Database) AddHistory(rec HistoryRecord) error {
	_, err := d.db.Exec(
		`INSERT INTO download_history
		 (url, url_type, download_time, total_count, success_count, config)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		rec.URL, rec.URLType, time.Now().Unix(),
		rec.TotalCount, rec.SuccessCount, rec.Config,
	)
	return err
}

// ---------------- job persistence ----------------

// JobRecord mirrors the task-center DownloadJob.to_dict() shape persisted to
// the job table. JSON-encoded fields are passed as strings (or empty).
type JobRecord struct {
	JobID            string
	URL              string
	Status           string
	CreatedAt        string
	StartedAt        string
	FinishedAt       string
	Total            int64
	Success          int64
	Failed           int64
	Skipped          int64
	Error            string
	AuthorNickname   string
	AuthorSecUID     string
	RetryCount       int64
	LastRetryAt      string
	LastRetrySummary any // marshaled to JSON
	RetryHistory     any // marshaled to JSON
	Overrides        any // marshaled to JSON
}

// UpsertJob inserts or replaces a terminal job row. JSON-shaped fields
// (last_retry_summary, retry_history, overrides) are JSON-encoded when non-nil.
func (d *Database) UpsertJob(rec JobRecord) error {
	lastRetrySummary, err := jsonOrNull(rec.LastRetrySummary)
	if err != nil {
		return fmt.Errorf("marshal last_retry_summary: %w", err)
	}
	retryHistory, err := jsonOrNull(rec.RetryHistory)
	if err != nil {
		return fmt.Errorf("marshal retry_history: %w", err)
	}
	overrides, err := jsonOrNull(rec.Overrides)
	if err != nil {
		return fmt.Errorf("marshal overrides: %w", err)
	}

	_, err = d.db.Exec(
		`INSERT OR REPLACE INTO job (
			job_id, url, status, created_at, started_at, finished_at,
			total, success, failed, skipped, error,
			author_nickname, author_sec_uid,
			retry_count, last_retry_at, last_retry_summary,
			retry_history, overrides
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.JobID, rec.URL, rec.Status, rec.CreatedAt,
		nullableStr(rec.StartedAt), nullableStr(rec.FinishedAt),
		rec.Total, rec.Success, rec.Failed, rec.Skipped,
		nullableStr(rec.Error),
		nullableStr(rec.AuthorNickname), nullableStr(rec.AuthorSecUID),
		rec.RetryCount, nullableStr(rec.LastRetryAt),
		lastRetrySummary, retryHistory, overrides,
	)
	return err
}

// ---------------- transcript_job ----------------

// TranscriptJobRecord mirrors the upsert payload for transcript_job.
type TranscriptJobRecord struct {
	AwemeID       string
	VideoPath     string
	TranscriptDir string
	TextPath      string
	JSONPath      string
	Model         string
	Status        string
	SkipReason    string
	ErrorMessage  string
}

// UpsertTranscriptJob upserts a transcript_job row on
// (aweme_id, video_path, model).
func (d *Database) UpsertTranscriptJob(rec TranscriptJobRecord) error {
	if rec.Model == "" {
		rec.Model = "gpt-4o-mini-transcribe"
	}
	now := time.Now().Unix()
	_, err := d.db.Exec(
		`INSERT INTO transcript_job (
			aweme_id, video_path, transcript_dir, text_path, json_path,
			model, status, skip_reason, error_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(aweme_id, video_path, model) DO UPDATE SET
			transcript_dir = excluded.transcript_dir,
			text_path      = excluded.text_path,
			json_path      = excluded.json_path,
			status         = excluded.status,
			skip_reason    = excluded.skip_reason,
			error_message  = excluded.error_message,
			updated_at     = excluded.updated_at`,
		nullableStr(rec.AwemeID), nullableStr(rec.VideoPath),
		nullableStr(rec.TranscriptDir), nullableStr(rec.TextPath),
		nullableStr(rec.JSONPath),
		rec.Model, rec.Status, nullableStr(rec.SkipReason),
		nullableStr(rec.ErrorMessage), now, now,
	)
	return err
}

// ---------------- helpers ----------------

// nullableStr returns nil for an empty string (→ SQL NULL), else the string.
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableInt returns nil when the NullInt64 is not valid.
func nullableInt(n sql.NullInt64) any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}

// jsonOrNull marshals v to indented JSON. Returns nil when v is nil.
func jsonOrNull(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// awemeUpsertSQL preserves the field-preserving upsert from database.py:
// empty incoming values never overwrite existing non-empty ones.
const awemeUpsertSQL = `
INSERT INTO aweme
  (aweme_id, aweme_type, title, author_id, author_name, author_sec_uid,
   create_time, download_time, file_path, metadata, cover_urls, job_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(aweme_id) DO UPDATE SET
  aweme_type = CASE WHEN COALESCE(excluded.aweme_type, '') != ''
                    THEN excluded.aweme_type
                    ELSE aweme.aweme_type END,
  title = CASE WHEN COALESCE(excluded.title, '') != ''
               THEN excluded.title
               ELSE aweme.title END,
  author_id = CASE WHEN COALESCE(excluded.author_id, '') != ''
                   THEN excluded.author_id
                   ELSE aweme.author_id END,
  author_name = CASE WHEN COALESCE(excluded.author_name, '') != ''
                     THEN excluded.author_name
                     ELSE aweme.author_name END,
  author_sec_uid = CASE WHEN COALESCE(excluded.author_sec_uid, '') != ''
                        THEN excluded.author_sec_uid
                        ELSE aweme.author_sec_uid END,
  create_time = CASE WHEN COALESCE(excluded.create_time, 0) != 0
                     THEN excluded.create_time
                     ELSE aweme.create_time END,
  download_time = CASE WHEN COALESCE(excluded.file_path, '') != ''
                       THEN excluded.download_time
                       ELSE aweme.download_time END,
  file_path = CASE WHEN COALESCE(excluded.file_path, '') != ''
                   THEN excluded.file_path
                   ELSE aweme.file_path END,
  metadata = CASE WHEN COALESCE(excluded.metadata, '') != ''
                  THEN excluded.metadata
                  ELSE aweme.metadata END,
  cover_urls = CASE WHEN excluded.cover_urls != '' AND excluded.cover_urls != '[]'
                    THEN excluded.cover_urls
                    ELSE aweme.cover_urls END,
  job_id = CASE WHEN excluded.job_id != ''
                THEN excluded.job_id
                ELSE aweme.job_id END`
