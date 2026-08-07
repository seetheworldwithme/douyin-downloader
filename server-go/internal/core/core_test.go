package core

import (
	"testing"
)

func TestParseURLVideo(t *testing.T) {
	p := ParseURL("https://www.douyin.com/video/7380308675841297704")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "video" {
		t.Errorf("expected type=video, got %s", p.Type)
	}
	if p.AwemeID != "7380308675841297704" {
		t.Errorf("expected aweme_id=7380308675841297704, got %s", p.AwemeID)
	}
}

func TestParseURLUser(t *testing.T) {
	p := ParseURL("https://www.douyin.com/user/MS4wLjABAAAA6O7EZyfDRYXxJrUTpf91K3tmB4rBROkAw")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "user" {
		t.Errorf("expected type=user, got %s", p.Type)
	}
	if p.SecUID != "MS4wLjABAAAA6O7EZyfDRYXxJrUTpf91K3tmB4rBROkAw" {
		t.Errorf("unexpected sec_uid: %s", p.SecUID)
	}
}

func TestParseURLCollection(t *testing.T) {
	p := ParseURL("https://www.douyin.com/collection/7234567890")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "collection" {
		t.Errorf("expected type=collection, got %s", p.Type)
	}
	if p.MixID != "7234567890" {
		t.Errorf("expected mix_id=7234567890, got %s", p.MixID)
	}
}

func TestParseURLGallery(t *testing.T) {
	p := ParseURL("https://www.douyin.com/note/7234567890")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "gallery" {
		t.Errorf("expected type=gallery, got %s", p.Type)
	}
	if p.NoteID != "7234567890" {
		t.Errorf("expected note_id=7234567890, got %s", p.NoteID)
	}
}

func TestParseURLMusic(t *testing.T) {
	p := ParseURL("https://www.douyin.com/music/7234567890")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "music" {
		t.Errorf("expected type=music, got %s", p.Type)
	}
}

func TestParseURLLive(t *testing.T) {
	p := ParseURL("https://live.douyin.com/123456789")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "live" {
		t.Errorf("expected type=live, got %s", p.Type)
	}
	if p.RoomID != "123456789" {
		t.Errorf("expected room_id=123456789, got %s", p.RoomID)
	}
}

func TestParseURLLiveReplay(t *testing.T) {
	p := ParseURL("https://www.douyin.com/vsdetail/7331203341890049058")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "live_replay" {
		t.Errorf("expected type=live_replay, got %s", p.Type)
	}
	if p.EpisodeID != "7331203341890049058" {
		t.Errorf("expected episode_id, got %s", p.EpisodeID)
	}
}

func TestParseURLModalID(t *testing.T) {
	p := ParseURL("https://www.douyin.com/discover?modal_id=7380308675841297704")
	if p == nil {
		t.Fatal("expected non-nil result")
	}
	if p.Type != "video" {
		t.Errorf("expected type=video for modal_id, got %s", p.Type)
	}
	if p.AwemeID != "7380308675841297704" {
		t.Errorf("expected aweme_id, got %s", p.AwemeID)
	}
}

func TestParseURLInvalid(t *testing.T) {
	p := ParseURL("https://example.com/foo")
	if p != nil {
		t.Error("expected nil for non-douyin URL")
	}
}

func TestPickPreferredPlayAddr(t *testing.T) {
	video := map[string]any{
		"play_addr": map[string]any{
			"url_list": []any{"https://example.com/video.mp4"},
		},
	}
	addr := pickPreferredPlayAddr(video, "highest")
	if addr == nil {
		t.Fatal("expected non-nil play_addr")
	}
	urls, _ := addr["url_list"].([]any)
	if len(urls) != 1 {
		t.Errorf("expected 1 url, got %d", len(urls))
	}
}

func TestPickRatio(t *testing.T) {
	tests := []struct {
		quality  string
		expected string
	}{
		{"highest", "1080p"},
		{"lowest", "540p"},
		{"720p", "720p"},
		{"invalid", "1080p"},
	}
	for _, tt := range tests {
		t.Run(tt.quality, func(t *testing.T) {
			if got := pickRatio(tt.quality); got != tt.expected {
				t.Errorf("pickRatio(%q) = %q, want %q", tt.quality, got, tt.expected)
			}
		})
	}
}
