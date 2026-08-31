package server

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xuziyue/douyin-downloader/internal/core"
	"github.com/xuziyue/douyin-downloader/internal/utils"
)

func selectedBatchItems(job *BatchJob, rawIDs string) []BatchItem {
	if job == nil {
		return nil
	}
	if strings.TrimSpace(rawIDs) == "" {
		return append([]BatchItem(nil), job.Items...)
	}
	wanted := map[string]bool{}
	for _, id := range strings.Split(rawIDs, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			wanted[id] = true
		}
	}
	out := make([]BatchItem, 0, len(wanted))
	for _, item := range job.Items {
		if wanted[item.AwemeID] {
			out = append(out, item)
		}
	}
	return out
}

func zipWriteBytes(zw *zip.Writer, name string, data []byte) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Store, Modified: time.Now()}
	hdr.SetMode(0o644)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func zipWriteError(zw *zip.Writer, item BatchItem, err error) {
	name := fmt.Sprintf("errors/%s.txt", item.AwemeID)
	_ = zipWriteBytes(zw, name, []byte(fmt.Sprintf("%s\n%s\n%v\n", item.Title, item.URL, err)))
}

// handleBatchStream streams selected results as a ZIP archive. Media is fetched
// from Douyin and written directly into the response; the server does not keep
// persistent copies. Gallery posts are stored as image folders inside the ZIP.
func (s *Server) handleBatchStream(w http.ResponseWriter, r *http.Request) {
	b := batchServiceFor(s)
	if b == nil {
		writeError(w, 503, "batch service unavailable")
		return
	}
	jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))
	if jobID == "" {
		writeError(w, 400, "job_id is required")
		return
	}
	job := b.snapshot(jobID)
	if job == nil {
		writeError(w, 404, "job not found")
		return
	}
	if len(job.Items) == 0 {
		writeError(w, 409, "该任务没有可下载的内存结果；如果服务已重启，请重新执行任务后再批量下载")
		return
	}
	items := selectedBatchItems(job, r.URL.Query().Get("ids"))
	if len(items) == 0 {
		writeError(w, 400, "没有选择可下载作品")
		return
	}

	setContentDisposition(w, "douyin_batch_"+job.JobID+".zip", "douyin_batch.zip")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")

	zw := zip.NewWriter(w)
	defer zw.Close()

	for i, item := range items {
		if err := r.Context().Err(); err != nil {
			return
		}
		res, err := s.resolveMedia(r.Context(), item.URL)
		if err != nil {
			zipWriteError(zw, item, err)
			continue
		}

		func() {
			defer res.APIClient.Close()
			if res.Type == "images" {
				folder := fmt.Sprintf("%03d_%s_%s", i+1, utils.SanitizeFilename(item.Title, 60), item.AwemeID)
				for imageIndex, image := range res.Images {
					data, ctype, imageErr := core.DownloadImage(r.Context(), res.APIClient, image)
					if imageErr != nil {
						zipWriteError(zw, item, fmt.Errorf("第 %d 张图片下载失败: %w", imageIndex+1, imageErr))
						return
					}
					ext := core.ImageExt(data, ctype)
					name := fmt.Sprintf("%s/%03d%s", folder, imageIndex+1, ext)
					if imageErr = zipWriteBytes(zw, name, data); imageErr != nil {
						zipWriteError(zw, item, imageErr)
						return
					}
				}
				return
			}

			info := res.Video
			if info == nil {
				zipWriteError(zw, item, fmt.Errorf("视频地址为空"))
				return
			}
			req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, info.VideoURL, nil)
			if reqErr != nil {
				zipWriteError(zw, item, reqErr)
				return
			}
			for k, v := range info.VideoHdrs {
				req.Header.Set(k, v)
			}
			resp, reqErr := res.APIClient.Client().Do(req)
			if reqErr != nil {
				zipWriteError(zw, item, reqErr)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				zipWriteError(zw, item, fmt.Errorf("上游返回 %d", resp.StatusCode))
				return
			}
			width := len(strconv.Itoa(len(items)))
			base := utils.SanitizeFilename(item.Title, 70)
			if base == "" {
				base = item.AwemeID
			}
			name := fmt.Sprintf("%0*d_%s_%s.mp4", width, i+1, base, item.AwemeID)
			hdr := &zip.FileHeader{Name: name, Method: zip.Store, Modified: time.Now()}
			hdr.SetMode(0o644)
			entry, createErr := zw.CreateHeader(hdr)
			if createErr != nil {
				return
			}
			if _, copyErr := io.Copy(entry, resp.Body); copyErr != nil {
				zipWriteError(zw, item, copyErr)
			}
		}()
	}
}
