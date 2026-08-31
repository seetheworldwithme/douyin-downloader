package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (d *Database) HasAweme(awemeID string) (bool, error) {
	var id int64
	err := d.db.QueryRow("SELECT id FROM aweme WHERE aweme_id = ?", awemeID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *Database) GetLatestSeenAwemeTime(secUID string) (int64, error) {
	var maxTime sql.NullInt64
	err := d.db.QueryRow("SELECT MAX(create_time) FROM aweme WHERE author_sec_uid = ?", secUID).Scan(&maxTime)
	if err != nil {
		return 0, err
	}
	if !maxTime.Valid {
		return 0, nil
	}
	return maxTime.Int64, nil
}

type RecentAweme struct {
	AwemeID      string `json:"aweme_id"`
	AwemeType    string `json:"type"`
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	AuthorSecUID string `json:"author_sec_uid"`
	CreateTime   int64  `json:"create_time"`
	DownloadTime int64  `json:"download_time"`
	FilePath     string `json:"file_path"`
	JobID        string `json:"job_id"`
	URL          string `json:"url"`
}

type HistoryFilter struct {
	Limit  int
	Offset int
	Query  string
	Type   string
	Author string
}

func (d *Database) ListAwemes(filter HistoryFilter) ([]RecentAweme, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"1=1"}
	args := make([]any, 0, 8)
	if q := strings.TrimSpace(filter.Query); q != "" {
		like := "%" + q + "%"
		where = append(where, "(COALESCE(title, '') LIKE ? OR COALESCE(author_name, '') LIKE ? OR aweme_id LIKE ?)")
		args = append(args, like, like, like)
	}
	if typ := strings.TrimSpace(filter.Type); typ != "" && typ != "all" {
		where = append(where, "aweme_type = ?")
		args = append(args, typ)
	}
	if author := strings.TrimSpace(filter.Author); author != "" {
		where = append(where, "COALESCE(author_name, '') LIKE ?")
		args = append(args, "%"+author+"%")
	}

	query := fmt.Sprintf(`
		SELECT aweme_id, aweme_type, COALESCE(title, ''), COALESCE(author_name, ''),
		       COALESCE(author_sec_uid, ''), COALESCE(create_time, 0),
		       COALESCE(download_time, 0), COALESCE(file_path, ''), COALESCE(job_id, '')
		FROM aweme
		WHERE %s
		ORDER BY COALESCE(create_time, download_time, 0) DESC, id DESC
		LIMIT ? OFFSET ?`, strings.Join(where, " AND "))
	args = append(args, filter.Limit, filter.Offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RecentAweme, 0, filter.Limit)
	for rows.Next() {
		var rec RecentAweme
		if err := rows.Scan(&rec.AwemeID, &rec.AwemeType, &rec.Title, &rec.AuthorName,
			&rec.AuthorSecUID, &rec.CreateTime, &rec.DownloadTime, &rec.FilePath, &rec.JobID); err != nil {
			return nil, err
		}
		rec.URL = "https://www.douyin.com/video/" + rec.AwemeID
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (d *Database) ListRecentAwemes(limit int) ([]RecentAweme, error) {
	return d.ListAwemes(HistoryFilter{Limit: limit})
}

type StoredJob struct {
	JobID          string `json:"job_id"`
	URL            string `json:"url"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	StartedAt      string `json:"started_at"`
	FinishedAt     string `json:"finished_at"`
	Total          int64  `json:"total"`
	Success        int64  `json:"success"`
	Failed         int64  `json:"failed"`
	Skipped        int64  `json:"skipped"`
	Error          string `json:"error"`
	AuthorNickname string `json:"author_nickname"`
	AuthorSecUID   string `json:"author_sec_uid"`
	Mode           string `json:"mode"`
	Incremental    bool   `json:"incremental"`
	MaxItems       int    `json:"max_items"`
}

func decodeJobOverrides(raw string, rec *StoredJob) {
	if rec == nil || strings.TrimSpace(raw) == "" {
		return
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return
	}
	if mode, ok := data["mode"].(string); ok {
		rec.Mode = mode
	}
	if inc, ok := data["incremental"].(bool); ok {
		rec.Incremental = inc
	}
	switch n := data["max_items"].(type) {
	case float64:
		rec.MaxItems = int(n)
	case int:
		rec.MaxItems = n
	}
	if rec.Mode == "" {
		rec.Mode = "post"
	}
	if rec.MaxItems <= 0 {
		rec.MaxItems = 50
	}
}

func scanStoredJob(scanner interface{ Scan(dest ...any) error }) (*StoredJob, error) {
	var rec StoredJob
	var overrides string
	if err := scanner.Scan(&rec.JobID, &rec.URL, &rec.Status, &rec.CreatedAt,
		&rec.StartedAt, &rec.FinishedAt, &rec.Total, &rec.Success, &rec.Failed,
		&rec.Skipped, &rec.Error, &rec.AuthorNickname, &rec.AuthorSecUID, &overrides); err != nil {
		return nil, err
	}
	decodeJobOverrides(overrides, &rec)
	return &rec, nil
}

func (d *Database) ListRecentJobs(limit int) ([]StoredJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.db.Query(`
		SELECT job_id, url, status, created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, ''),
		       total, success, failed, skipped, COALESCE(error, ''),
		       COALESCE(author_nickname, ''), COALESCE(author_sec_uid, ''),
		       COALESCE(overrides, '')
		FROM job ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StoredJob, 0, limit)
	for rows.Next() {
		rec, err := scanStoredJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	return out, rows.Err()
}

func (d *Database) GetJob(jobID string) (*StoredJob, error) {
	row := d.db.QueryRow(`
		SELECT job_id, url, status, created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, ''),
		       total, success, failed, skipped, COALESCE(error, ''),
		       COALESCE(author_nickname, ''), COALESCE(author_sec_uid, ''),
		       COALESCE(overrides, '')
		FROM job WHERE job_id = ?`, jobID)
	rec, err := scanStoredJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rec, err
}

func (d *Database) TouchHistory(rawURL, urlType string, total, success int64) error {
	if strings.TrimSpace(urlType) == "" {
		urlType = "user"
	}
	_, err := d.db.Exec(`
		INSERT INTO download_history
		(url, url_type, download_time, total_count, success_count, config)
		VALUES (?, ?, ?, ?, ?, '{}')`, rawURL, urlType, time.Now().Unix(), total, success)
	return err
}
