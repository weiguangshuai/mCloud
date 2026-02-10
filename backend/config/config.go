package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Storage    StorageConfig    `yaml:"storage"`
	Redis      RedisConfig      `yaml:"redis"`
	JWT        JWTConfig        `yaml:"jwt"`
	Thumbnail  ThumbnailConfig  `yaml:"thumbnail"`
	RecycleBin RecycleBinConfig `yaml:"recycle_bin"`
	Pagination PaginationConfig `yaml:"pagination"`
	Audit      AuditConfig      `yaml:"audit"`
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
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
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

type AuditConfig struct {
	Enabled       bool     `yaml:"enabled"`
	LogActions    []string `yaml:"log_actions"`
	RetentionDays int      `yaml:"retention_days"`
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

	AppConfig = &cfg
	return &cfg, nil
}
