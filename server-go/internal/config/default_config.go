package config

// Config 是服务端实际消费的全部配置项。
// ponytail: Python 时代的 ~30 个批量下载配置项(thread/mode/transcript/…)已随
// 批量功能一起删除;YAML 里多余的键会被静默忽略,旧 config.yml 无需迁移。

// Config is the top-level configuration.
type Config struct {
	Proxy        string   `yaml:"proxy"`
	VideoQuality string   `yaml:"video_quality"`
	FFmpegPath   string   `yaml:"ffmpeg_path"`
	AutoCookie   any      `yaml:"auto_cookie"`
	Cookies      any      `yaml:"cookies,omitempty"`
	Cookie       string   `yaml:"cookie,omitempty"`
	Server       ServerCfg `yaml:"server"`
	Auth         AuthCfg  `yaml:"auth"`
}

type ServerCfg struct {
	CorsOrigins []string `yaml:"cors_origins"`
}

type AuthCfg struct {
	Username string              `yaml:"username"`
	Password string              `yaml:"password"`
	Secret   string              `yaml:"secret"`
	Users    []map[string]string `yaml:"users"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		VideoQuality: "highest",
		Auth: AuthCfg{
			Username: "xuziyue",
			Password: "mmjsxu666555",
		},
	}
}
