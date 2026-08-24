package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Slideshow composer: turns gallery images (plus the post's background music
// when available) into an MP4 via a locally installed ffmpeg binary.

const (
	slideshowFPS        = 30
	slideshowMaxLongEdge  = 1440
	slideshowMaxShortEdge = 1920
	slideshowMaxTotal   = 10 * time.Minute
	defaultPerImage     = 3 * time.Second
	minPerImage         = 1 * time.Second
)

// SlideshowImage is one on-disk frame of the slideshow.
type SlideshowImage struct {
	Path   string
	Width  int
	Height int
}

// FindFFmpeg resolves the ffmpeg binary: an explicitly configured path wins,
// otherwise the PATH is searched.
func FindFFmpeg(configured string) (string, error) {
	if p := strings.TrimSpace(configured); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("配置的 ffmpeg_path 不存在: %s", p)
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("未找到 ffmpeg")
}

// SlideshowPerImageDuration decides how long each image stays on screen:
// with music the images are spread evenly across the full track (capped at
// 10 minutes); without music each image shows for a fixed 3 seconds.
func SlideshowPerImageDuration(imageCount int, musicDurationMS int64) time.Duration {
	if imageCount < 1 {
		imageCount = 1
	}
	if musicDurationMS <= 0 {
		return defaultPerImage
	}
	total := time.Duration(musicDurationMS) * time.Millisecond
	if total > slideshowMaxTotal {
		total = slideshowMaxTotal
	}
	d := total / time.Duration(imageCount)
	if d < minPerImage {
		d = minPerImage
	}
	return d
}

// slideshowCanvas picks the output canvas from the largest image, scaled down
// to the long/short edge limits. Images with a different aspect ratio are
// letterboxed by the pad filter instead of being distorted.
func slideshowCanvas(images []SlideshowImage) (int, int) {
	var bw, bh, best int
	for _, im := range images {
		if im.Width <= 0 || im.Height <= 0 {
			continue
		}
		if area := im.Width * im.Height; area > best {
			best, bw, bh = area, im.Width, im.Height
		}
	}
	if best == 0 {
		return 1080, 1440
	}
	scale := 1.0
	long, short := bw, bh
	if short > long {
		long, short = short, long
	}
	if long > slideshowMaxLongEdge {
		scale = float64(slideshowMaxLongEdge) / float64(long)
	}
	if short > slideshowMaxShortEdge {
		if s := float64(slideshowMaxShortEdge) / float64(short); s < scale {
			scale = s
		}
	}
	return even(int(float64(bw)*scale)), even(int(float64(bh)*scale))
}

func even(v int) int {
	if v < 2 {
		return 2
	}
	return v - v%2
}

// buildConcatScript renders an ffconcat playlist. The last file is repeated
// (a concat-demuxer requirement, otherwise the final frame has no duration).
func buildConcatScript(names []string, perImage time.Duration) string {
	var b strings.Builder
	b.WriteString("ffconcat version 1.0\n")
	duration := fmt.Sprintf("%.3f", perImage.Seconds())
	for _, name := range names {
		fmt.Fprintf(&b, "file '%s'\nduration %s\n", name, duration)
	}
	fmt.Fprintf(&b, "file '%s'\n", names[len(names)-1])
	return b.String()
}

// ComposeSlideshow encodes the images into workDir/out.mp4. audioPath may be
// empty for a silent slideshow. All image paths must live inside workDir so
// the concat playlist can reference them with -safe 0 relative names.
func ComposeSlideshow(ctx context.Context, ffmpegPath, workDir string, images []SlideshowImage, audioPath string, perImage time.Duration) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("没有可合成的图片")
	}
	if perImage <= 0 {
		perImage = defaultPerImage
	}

	names := make([]string, len(images))
	for i, im := range images {
		names[i] = filepath.Base(im.Path)
	}
	concatPath := filepath.Join(workDir, "slides.ffconcat")
	if err := os.WriteFile(concatPath, []byte(buildConcatScript(names, perImage)), 0o644); err != nil {
		return "", fmt.Errorf("写入 concat 脚本失败: %w", err)
	}

	w, h := slideshowCanvas(images)
	videoFilter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1,fps=%d,format=yuv420p",
		w, h, w, h, slideshowFPS)

	outPath := filepath.Join(workDir, "out.mp4")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", "slides.ffconcat",
	}
	if audioPath != "" {
		args = append(args, "-i", filepath.Base(audioPath))
	}
	args = append(args,
		"-vf", videoFilter,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
	)
	if audioPath != "" {
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-shortest")
	}
	args = append(args, "-movflags", "+faststart", outPath)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	cmd.Dir = workDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		tail := stderr.String()
		if len(tail) > 2000 {
			tail = tail[len(tail)-2000:]
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("合成已取消: %w", ctx.Err())
		}
		return "", fmt.Errorf("ffmpeg 执行失败: %v: %s", err, strings.TrimSpace(tail))
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		return "", fmt.Errorf("ffmpeg 未产出视频文件")
	}
	return outPath, nil
}
