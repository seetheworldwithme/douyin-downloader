package core

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func sampleGalleryAweme() map[string]any {
	return map[string]any{
		"desc": ".sample 图集",
		"images": []any{
			map[string]any{
				"url_list":   []any{"https://p3.douyinpic.com/img1.jpeg", "https://p9.douyinpic.com/img1.jpeg"},
				"width":      float64(1080),
				"height":     float64(1440),
				"download_url_list": []any{"https://p3.douyinpic.com/img1_dl.jpeg"},
			},
			map[string]any{
				"url_list": []any{"https://p3.douyinpic.com/img2.jpeg"},
				"width":    float64(1080),
				"height":   float64(1440),
			},
			map[string]any{
				// 无 URL 的条目应被跳过
				"url_list": []any{},
			},
		},
		"music": map[string]any{
			"duration": float64(20440),
			"play_url": map[string]any{
				"url_list": []any{"https://sf3-cdn-tos.douyinstatic.com/a.mp3", ""},
			},
		},
	}
}

func TestExtractImages(t *testing.T) {
	images := extractImages(sampleGalleryAweme())
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}
	if got := images[0].URLs; len(got) != 3 {
		t.Errorf("expected 3 candidate URLs (url_list + download_url_list), got %v", got)
	}
	if images[0].Width != 1080 || images[0].Height != 1440 {
		t.Errorf("unexpected dims: %dx%d", images[0].Width, images[0].Height)
	}
	if images := extractImages(map[string]any{"desc": "video"}); images != nil {
		t.Errorf("video post should yield no images, got %v", images)
	}
}

func TestExtractMusic(t *testing.T) {
	urls, durationMS := extractMusic(sampleGalleryAweme())
	if len(urls) != 1 {
		t.Fatalf("expected 1 music URL, got %v", urls)
	}
	if durationMS != 20440 {
		t.Errorf("expected duration 20440ms, got %d", durationMS)
	}
}

func TestSlideshowCanvas(t *testing.T) {
	cases := []struct {
		images []SlideshowImage
		w, h   int
	}{
		// 最大面积图决定画布,保持偶数
		{[]SlideshowImage{{Width: 1080, Height: 1440}, {Width: 800, Height: 600}}, 1080, 1440},
		// 超长边缩到 1440,等比
		{[]SlideshowImage{{Width: 2160, Height: 3840}}, 810, 1440},
		// 横图
		{[]SlideshowImage{{Width: 4032, Height: 3024}}, 1440, 1080},
		// 无有效尺寸 → 默认
		{[]SlideshowImage{{}}, 1080, 1440},
	}
	for _, c := range cases {
		w, h := slideshowCanvas(c.images)
		if w != c.w || h != c.h {
			t.Errorf("slideshowCanvas(%v) = %dx%d, want %dx%d", c.images, w, h, c.w, c.h)
		}
	}
}

func TestBuildConcatScript(t *testing.T) {
	got := buildConcatScript([]string{"img_000.jpg", "img_001.jpg"}, 3*time.Second)
	want := "ffconcat version 1.0\n" +
		"file 'img_000.jpg'\nduration 3.000\n" +
		"file 'img_001.jpg'\nduration 3.000\n" +
		"file 'img_001.jpg'\n"
	if got != want {
		t.Errorf("concat script mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSlideshowPerImageDuration(t *testing.T) {
	// 有音乐:总时长均分
	if d := SlideshowPerImageDuration(4, 20000); d != 5*time.Second {
		t.Errorf("music spread: got %v, want 5s", d)
	}
	// 图片太多时每张至少 1s
	if d := SlideshowPerImageDuration(30, 20000); d != 1*time.Second {
		t.Errorf("min per image: got %v, want 1s", d)
	}
	// 超长音乐封顶 10 分钟
	if d := SlideshowPerImageDuration(2, 40*60*1000); d != 5*time.Minute {
		t.Errorf("total cap: got %v, want 5m", d)
	}
	// 无音乐:固定 3s
	if d := SlideshowPerImageDuration(3, 0); d != 3*time.Second {
		t.Errorf("no music: got %v, want 3s", d)
	}
}

func TestImageExt(t *testing.T) {
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	pngHeader := append([]byte{0x89}, []byte("PNG\r\n\x1a\n")...)
	webpHeader := append([]byte("RIFF\x00\x00\x00\x00"), []byte("WEBP")...)

	if ext := ImageExt(jpegHeader, "application/octet-stream"); ext != ".jpg" {
		t.Errorf("jpeg sniff failed: %s", ext)
	}
	if ext := ImageExt(pngHeader, ""); ext != ".png" {
		t.Errorf("png sniff failed: %s", ext)
	}
	if ext := ImageExt(webpHeader, ""); ext != ".webp" {
		t.Errorf("webp sniff failed: %s", ext)
	}
	if ext := ImageExt([]byte("xxxx"), "image/png"); ext != ".png" {
		t.Errorf("content-type fallback failed: %s", ext)
	}
	if ext := ImageExt([]byte("xxxx"), ""); ext != ".jpg" {
		t.Errorf("default ext failed: %s", ext)
	}
}

func TestAudioExt(t *testing.T) {
	if ext := AudioExt([]byte("ID3\x04\x00")); ext != ".mp3" {
		t.Errorf("mp3 sniff failed: %s", ext)
	}
	if ext := AudioExt([]byte{0xFF, 0xFB, 0x90, 0x00}); ext != ".mp3" {
		t.Errorf("mpeg frame sniff failed: %s", ext)
	}
	if ext := AudioExt([]byte("....ftypM4A ")); ext != ".m4a" {
		t.Errorf("m4a sniff failed: %s", ext)
	}
}

// TestComposeSlideshowWithFFmpeg is an integration test that only runs when a
// local ffmpeg binary is available.
func TestComposeSlideshowWithFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed, skipping integration test")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "img_000.jpg"), testJPEG(t, 640, 480, 255, 0, 0), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "img_001.jpg"), testJPEG(t, 320, 240, 0, 0, 255), 0o644); err != nil {
		t.Fatal(err)
	}

	images := []SlideshowImage{
		{Path: filepath.Join(dir, "img_000.jpg"), Width: 640, Height: 480},
		{Path: filepath.Join(dir, "img_001.jpg"), Width: 320, Height: 240},
	}
	out, err := ComposeSlideshow(t.Context(), ffmpeg, dir, images, "", time.Second)
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	st, err := os.Stat(out)
	if err != nil || st.Size() == 0 {
		t.Fatalf("output missing or empty: %v", err)
	}
}

// testJPEG encodes a solid-color JPEG of the given size.
func testJPEG(t *testing.T, w, h, r, g, b int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg encode failed: %v", err)
	}
	return buf.Bytes()
}
