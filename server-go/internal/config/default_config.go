package config

// Config mirrors the Python DEFAULT_CONFIG. Fields use pointer types for
// proper YAML omitempty semantics where needed, and maps for dynamic sections.

// Config is the top-level configuration.
type Config struct {
	Path             string             `yaml:"path"`
	Music            bool               `yaml:"music"`
	Cover            bool               `yaml:"cover"`
	Avatar           bool               `yaml:"avatar"`
	JSON             bool               `yaml:"json"`
	StartTime        string             `yaml:"start_time"`
	EndTime          string             `yaml:"end_time"`
	FolderStyle      bool               `yaml:"folderstyle"`
	FilenameTemplate string             `yaml:"filename_template"`
	FolderTemplate   string             `yaml:"folder_template"`
	AuthorDir        string             `yaml:"author_dir"`
	GroupByMode      bool               `yaml:"group_by_mode"`
	DownloadPinned   bool               `yaml:"download_pinned"`
	Mode             []string           `yaml:"mode"`
	Number           map[string]int     `yaml:"number"`
	Increase         map[string]bool    `yaml:"increase"`
	Thread           int                `yaml:"thread"`
	RetryTimes       int                `yaml:"retry_times"`
	RateLimit        float64            `yaml:"rate_limit"`
	Proxy            string             `yaml:"proxy"`
	VideoQuality     string             `yaml:"video_quality"`
	Database         bool               `yaml:"database"`
	DatabasePath     string             `yaml:"database_path"`
	Progress         map[string]any     `yaml:"progress"`
	Transcript       TranscriptConfig   `yaml:"transcript"`
	AutoCookie       any                `yaml:"auto_cookie"`
	BrowserFallback  BrowserFallbackCfg `yaml:"browser_fallback"`
	Notifications    NotificationsCfg   `yaml:"notifications"`
	Comments         CommentsCfg        `yaml:"comments"`
	Live             LiveCfg            `yaml:"live"`
	Server           ServerCfg          `yaml:"server"`
	Auth             AuthCfg            `yaml:"auth"`
	Cookies          any                `yaml:"cookies,omitempty"`
	Cookie           string             `yaml:"cookie,omitempty"`
	Link             []string           `yaml:"link"`
}

type TranscriptConfig struct {
	Enabled          bool     `yaml:"enabled"`
	Model            string   `yaml:"model"`
	OutputDir        string   `yaml:"output_dir"`
	ResponseFormats  []string `yaml:"response_formats"`
	APIURL           string   `yaml:"api_url"`
	APIKeyEnv        string   `yaml:"api_key_env"`
	APIKey           string   `yaml:"api_key"`
	UploadAudioOnly  bool     `yaml:"upload_audio_only"`
}

type BrowserFallbackCfg struct {
	Enabled            bool `yaml:"enabled"`
	Headless           bool `yaml:"headless"`
	MaxScrolls         int  `yaml:"max_scrolls"`
	IdleRounds         int  `yaml:"idle_rounds"`
	WaitTimeoutSeconds int  `yaml:"wait_timeout_seconds"`
}

type NotificationsCfg struct {
	Enabled   bool           `yaml:"enabled"`
	OnSuccess bool           `yaml:"on_success"`
	OnFailure bool           `yaml:"on_failure"`
	Providers []map[string]any `yaml:"providers"`
}

type CommentsCfg struct {
	Enabled        bool `yaml:"enabled"`
	IncludeReplies bool `yaml:"include_replies"`
	MaxComments    int  `yaml:"max_comments"`
	PageSize       int  `yaml:"page_size"`
}

type LiveCfg struct {
	MaxDurationSeconds int `yaml:"max_duration_seconds"`
	ChunkSize          int `yaml:"chunk_size"`
	IdleTimeoutSeconds int `yaml:"idle_timeout_seconds"`
}

type ServerCfg struct {
	MaxJobs       int      `yaml:"max_jobs"`
	JobTTLSeconds int      `yaml:"job_ttl_seconds"`
	CorsOrigins   []string `yaml:"cors_origins"`
}

type AuthCfg struct {
	Username string         `yaml:"username"`
	Password string         `yaml:"password"`
	Secret   string         `yaml:"secret"`
	Users    []map[string]string `yaml:"users"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Path:             "./Downloaded/",
		Music:            true,
		Cover:            true,
		Avatar:           true,
		JSON:             true,
		StartTime:        "",
		EndTime:          "",
		FolderStyle:      true,
		FilenameTemplate: "{date}_{title}_{id}",
		FolderTemplate:   "{date}_{title}_{id}",
		AuthorDir:        "nickname",
		GroupByMode:      true,
		DownloadPinned:   false,
		Mode:             []string{"post"},
		Number: map[string]int{
			"post": 0, "like": 0, "allmix": 0, "mix": 0,
			"music": 0, "collect": 0, "collectmix": 0,
		},
		Increase: map[string]bool{
			"post": false, "like": false, "allmix": false,
			"mix": false, "music": false,
		},
		Thread:           5,
		RetryTimes:       3,
		RateLimit:        2,
		Proxy:            "",
		VideoQuality:     "highest",
		Database:         true,
		DatabasePath:     "dy_downloader.db",
		Progress:         map[string]any{"quiet_logs": true},
		AutoCookie:       false,
		BrowserFallback: BrowserFallbackCfg{
			Enabled: true, Headless: false, MaxScrolls: 240,
			IdleRounds: 8, WaitTimeoutSeconds: 600,
		},
		Notifications: NotificationsCfg{
			Enabled: false, OnSuccess: true, OnFailure: true,
			Providers: []map[string]any{},
		},
		Comments: CommentsCfg{
			Enabled: false, IncludeReplies: false,
			MaxComments: 0, PageSize: 20,
		},
		Live: LiveCfg{
			MaxDurationSeconds: 0, ChunkSize: 65536, IdleTimeoutSeconds: 30,
		},
		Transcript: TranscriptConfig{
			Enabled:         false,
			Model:           "gpt-4o-mini-transcribe",
			OutputDir:       "",
			ResponseFormats: []string{"txt", "json"},
			APIURL:          "https://api.openai.com/v1/audio/transcriptions",
			APIKeyEnv:       "OPENAI_API_KEY",
			APIKey:          "",
			UploadAudioOnly: true,
		},
		Server: ServerCfg{
			MaxJobs: 500, JobTTLSeconds: 86400,
		},
		Auth: AuthCfg{
			Username: "xuziyue",
			Password: "mmjsxu666555",
			Secret:   "",
			Users:    []map[string]string{},
		},
		Link: []string{},
	}
}
