package storage

import (
	"database/sql"
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
}

func (d *Database) ListRecentAwemes(limit int) ([]RecentAweme, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := d.db.Query(`
		SELECT aweme_id, aweme_type, COALESCE(title, ''), COALESCE(author_name, ''),
		       COALESCE(author_sec_uid, ''), COALESCE(create_time, 0),
		       COALESCE(download_time, 0), COALESCE(file_path, ''), COALESCE(job_id, '')
		FROM aweme
		ORDER BY COALESCE(create_time, download_time, 0) DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RecentAweme, 0, limit)
	for rows.Next() {
		var rec RecentAweme
		if err := rows.Scan(&rec.AwemeID, &rec.AwemeType, &rec.Title, &rec.AuthorName,
			&rec.AuthorSecUID, &rec.CreateTime, &rec.DownloadTime, &rec.FilePath, &rec.JobID); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
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
}

func (d *Database) ListRecentJobs(limit int) ([]StoredJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.db.Query(`
		SELECT job_id, url, status, created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, ''),
		       total, success, failed, skipped, COALESCE(error, ''),
		       COALESCE(author_nickname, ''), COALESCE(author_sec_uid, '')
		FROM job ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StoredJob, 0, limit)
	for rows.Next() {
		var rec StoredJob
		if err := rows.Scan(&rec.JobID, &rec.URL, &rec.Status, &rec.CreatedAt,
			&rec.StartedAt, &rec.FinishedAt, &rec.Total, &rec.Success, &rec.Failed,
			&rec.Skipped, &rec.Error, &rec.AuthorNickname, &rec.AuthorSecUID); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (d *Database) TouchHistory(rawURL string, total, success int64) error {
	_, err := d.db.Exec(`
		INSERT INTO download_history
		(url, url_type, download_time, total_count, success_count, config)
		VALUES (?, 'user', ?, ?, ?, '{}')`, rawURL, time.Now().Unix(), total, success)
	return err
}
