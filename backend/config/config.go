package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig      `yaml:"server"`
	Database   DatabaseConfig    `yaml:"database"`
	Storage    StorageConfig     `yaml:"storage"`
	Redis      RedisConfig       `yaml:"redis"`
	JWT        JWTConfig         `yaml:"jwt"`
	AuthCookie AuthCookieConfig  `yaml:"auth_cookie"`
	CSRF       CSRFConfig        `yaml:"csrf"`
	Thumbnail  ThumbnailConfig   `yaml:"thumbnail"`
	RecycleBin RecycleBinConfig  `yaml:"recycle_bin"`
	Pagination PaginationConfig  `yaml:"pagination"`
	Health     HealthCheckConfig `yaml:"health_check"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	Charset      string `yaml:"charset"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns"`
}

type StorageConfig struct {
	BasePath                string   `yaml:"base_path"`
	MaxFileSize             int64    `yaml:"max_file_size"`
	AllowedExtensions       []string `yaml:"allowed_extensions"`
	ChunkSize               int64    `yaml:"chunk_size"`
	ChunkUploadThreshold    int64    `yaml:"chunk_upload_threshold"`
	DefaultUserQuota        int64    `yaml:"default_user_quota"`
	TempFileCleanupInterval int      `yaml:"temp_file_cleanup_interval"`
	TempFileRetention       int      `yaml:"temp_file_retention"`
}

type RedisConfig struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port"`
	Password         string `yaml:"password"`
	DB               int    `yaml:"db"`
	UploadTaskExpire int    `yaml:"upload_task_expire"`
}

type JWTConfig struct {
	Secret             string `yaml:"secret"`
	ExpireHours        int    `yaml:"expire_hours"`
	RefreshExpireHours int    `yaml:"refresh_expire_hours"`
}

type AuthCookieConfig struct {
	AccessName  string `yaml:"access_name"`
	RefreshName string `yaml:"refresh_name"`
	HttpOnly    bool   `yaml:"http_only"`
	SameSite    string `yaml:"same_site"`
	Secure      bool   `yaml:"secure"`
	Path        string `yaml:"path"`
}

type CSRFConfig struct {
	Enabled    bool   `yaml:"enabled"`
	HeaderName string `yaml:"header_name"`
	CookieName string `yaml:"cookie_name"`
}

type ThumbnailConfig struct {
	Width           int  `yaml:"width"`
	Height          int  `yaml:"height"`
	Quality         int  `yaml:"quality"`
	AsyncGeneration bool `yaml:"async_generation"`
	WorkerCount     int  `yaml:"worker_count"`
	RetryMax        int  `yaml:"retry_max"`
}

type RecycleBinConfig struct {
	Enabled         bool `yaml:"enabled"`
	RetentionDays   int  `yaml:"retention_days"`
	CleanupInterval int  `yaml:"cleanup_interval"`
	MaxItemsPerUser int  `yaml:"max_items_per_user"`
}

type PaginationConfig struct {
	DefaultPageSize int    `yaml:"default_page_size"`
	MaxPageSize     int    `yaml:"max_page_size"`
	DefaultSortBy   string `yaml:"default_sort_by"`
	DefaultOrder    string `yaml:"default_order"`
}

type HealthCheckConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Endpoint  string `yaml:"endpoint"`
	TimeoutMs int    `yaml:"timeout_ms"`
}

var AppConfig *Config

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyDefaults(&cfg)

	AppConfig = &cfg
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.AuthCookie.AccessName == "" {
		cfg.AuthCookie.AccessName = "access_token"
	}
	if cfg.AuthCookie.RefreshName == "" {
		cfg.AuthCookie.RefreshName = "refresh_token"
	}
	if cfg.AuthCookie.Path == "" {
		cfg.AuthCookie.Path = "/"
	}
	if cfg.CSRF.HeaderName == "" {
		cfg.CSRF.HeaderName = "X-CSRF-Token"
	}
	if cfg.CSRF.CookieName == "" {
		cfg.CSRF.CookieName = "csrf_token"
	}
	if cfg.JWT.RefreshExpireHours == 0 {
		if cfg.JWT.ExpireHours > 0 {
			cfg.JWT.RefreshExpireHours = cfg.JWT.ExpireHours * 4
		} else {
			cfg.JWT.RefreshExpireHours = 168
		}
	}
}
