package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
)

type Server struct {
	HTTPPort          int           `koanf:"http_port"`
	ManagementPort    int           `koanf:"management_port"`
	ReadHeaderTimeout time.Duration `koanf:"read_header_timeout"`
	ReadTimeout       time.Duration `koanf:"read_timeout"`
	WriteTimeout      time.Duration `koanf:"write_timeout"`
	IdleTimeout       time.Duration `koanf:"idle_timeout"`
	ShutdownTimeout   time.Duration `koanf:"shutdown_timeout"`
	MaxHeaderBytes    int           `koanf:"max_header_bytes"`
	MaxBodyBytes      int64         `koanf:"max_body_bytes"`
}
type MySQL struct {
	DSN             string        `koanf:"dsn"`
	MaxOpenConns    int           `koanf:"max_open_conns"`
	MaxIdleConns    int           `koanf:"max_idle_conns"`
	ConnMaxLifetime time.Duration `koanf:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `koanf:"conn_max_idle_time"`
}
type Redis struct {
	Address      string        `koanf:"address"`
	Password     string        `koanf:"password"`
	Database     int           `koanf:"database"`
	DialTimeout  time.Duration `koanf:"dial_timeout"`
	ReadTimeout  time.Duration `koanf:"read_timeout"`
	WriteTimeout time.Duration `koanf:"write_timeout"`
}
type Identity struct {
	AccessTokenTTL  time.Duration `koanf:"access_token_ttl"`
	RefreshTokenTTL time.Duration `koanf:"refresh_token_ttl"`
	CaptchaTTL      time.Duration `koanf:"captcha_ttl"`
	BootstrapUser   string        `koanf:"bootstrap_user"`
	BootstrapPass   string        `koanf:"bootstrap_password"`
	DefaultPassword string        `koanf:"default_password"`
}
type FileStorage struct {
	Type          string           `koanf:"type"`
	Root          string           `koanf:"root"`
	PublicBaseURL string           `koanf:"public_base_url"`
	MaxFileBytes  int64            `koanf:"max_file_bytes"`
	S3            S3Storage        `koanf:"s3"`
	AliyunOSS     AliyunOSSStorage `koanf:"aliyun_oss"`
}
type S3Storage struct {
	Endpoint     string `koanf:"endpoint"`
	Region       string `koanf:"region"`
	Bucket       string `koanf:"bucket"`
	AccessKey    string `koanf:"access_key"`
	SecretKey    string `koanf:"secret_key"`
	UsePathStyle bool   `koanf:"use_path_style"`
}
type AliyunOSSStorage struct {
	Endpoint  string `koanf:"endpoint"`
	Bucket    string `koanf:"bucket"`
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
}
type RocketMQ struct {
	Endpoints      string        `koanf:"endpoints"`
	AccessKey      string        `koanf:"access_key"`
	SecretKey      string        `koanf:"secret_key"`
	TopicPrefix    string        `koanf:"topic_prefix"`
	ConsumerGroup  string        `koanf:"consumer_group"`
	AwaitDuration  time.Duration `koanf:"await_duration"`
	HandlerTimeout time.Duration `koanf:"handler_timeout"`
	Topics         []string      `koanf:"topics"`
	Concurrency    int32         `koanf:"concurrency"`
	MaxBodyBytes   int           `koanf:"max_body_bytes"`
}

type Role struct {
	Environment string         `koanf:"environment"`
	Server      Server         `koanf:"server"`
	Logging     logging.Config `koanf:"logging"`
	MySQL       MySQL          `koanf:"mysql"`
	Redis       Redis          `koanf:"redis"`
	Identity    Identity       `koanf:"identity"`
	FileStorage FileStorage    `koanf:"file_storage"`
	RocketMQ    RocketMQ       `koanf:"rocketmq"`
}

func Defaults() Role {
	return Role{
		Environment: "local",
		Server: Server{
			HTTPPort: 8080, ManagementPort: 9090,
			ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
			WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
			ShutdownTimeout: 20 * time.Second, MaxHeaderBytes: 1 << 20, MaxBodyBytes: 2 << 20,
		},
		Logging:     logging.Config{Level: "info", Format: "text"},
		MySQL:       MySQL{MaxOpenConns: 25, MaxIdleConns: 10, ConnMaxLifetime: 30 * time.Minute, ConnMaxIdleTime: 5 * time.Minute},
		Redis:       Redis{Address: "127.0.0.1:6379", DialTimeout: 3 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second},
		Identity:    Identity{AccessTokenTTL: 2 * time.Hour, RefreshTokenTTL: 7 * 24 * time.Hour, CaptchaTTL: 5 * time.Minute},
		FileStorage: FileStorage{Type: "local", Root: ".tmp/uploads", MaxFileBytes: 2 << 20, S3: S3Storage{Region: "us-east-1", UsePathStyle: true}},
		RocketMQ:    RocketMQ{HandlerTimeout: 30 * time.Second},
	}
}

func Path(role string) string {
	if path := os.Getenv("APP_CONFIG_FILE"); path != "" {
		return path
	}
	return "configs/" + role + ".yaml"
}

func Load(path, role string, target *Role) error {
	loader := koanf.New(".")
	if err := loader.Load(file.Provider(path), yaml.Parser()); err != nil {
		return fmt.Errorf("load config file %s: %w", path, err)
	}
	if err := loader.Load(env.Provider("APP_", ".", mapEnvironmentKey), nil); err != nil {
		return fmt.Errorf("load environment config: %w", err)
	}
	if err := loader.UnmarshalWithConf("", target, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return target.Validate(role)
}

func (c Role) Validate(role string) error {
	if role != "api" && role != "job" && role != "consumer" {
		return fmt.Errorf("unknown role %q", role)
	}
	if role == "api" && c.Server.HTTPPort <= 0 {
		return fmt.Errorf("server.http_port is required for the api role")
	}
	if role == "api" {
		if strings.TrimSpace(c.Redis.Address) == "" {
			return fmt.Errorf("redis.address is required for local identity")
		}
		if c.Identity.AccessTokenTTL <= 0 || c.Identity.RefreshTokenTTL <= 0 || c.Identity.CaptchaTTL <= 0 {
			return fmt.Errorf("identity token and captcha TTLs must be positive")
		}
		bootstrapUserConfigured := strings.TrimSpace(c.Identity.BootstrapUser) != ""
		bootstrapPasswordConfigured := c.Identity.BootstrapPass != ""
		if bootstrapUserConfigured != bootstrapPasswordConfigured {
			return fmt.Errorf("identity bootstrap username and password must be configured together")
		}
		if bootstrapPasswordConfigured && len(c.Identity.BootstrapPass) < 8 {
			return fmt.Errorf("identity bootstrap password must contain at least 8 characters")
		}
		if c.Identity.DefaultPassword != "" && len(c.Identity.DefaultPassword) < 8 {
			return fmt.Errorf("identity default password must contain at least 8 characters")
		}
		if c.FileStorage.MaxFileBytes <= 0 || c.FileStorage.MaxFileBytes > c.Server.MaxBodyBytes {
			return fmt.Errorf("file_storage.max_file_bytes is required and must not exceed server.max_body_bytes")
		}
		switch strings.ToLower(strings.TrimSpace(c.FileStorage.Type)) {
		case "local":
			if strings.TrimSpace(c.FileStorage.Root) == "" {
				return fmt.Errorf("file_storage.root is required for local storage")
			}
		case "s3":
			if strings.TrimSpace(c.FileStorage.S3.Bucket) == "" || strings.TrimSpace(c.FileStorage.S3.Region) == "" {
				return fmt.Errorf("file_storage.s3 bucket and region are required")
			}
			if (strings.TrimSpace(c.FileStorage.S3.AccessKey) == "") != (strings.TrimSpace(c.FileStorage.S3.SecretKey) == "") {
				return fmt.Errorf("file_storage.s3 access_key and secret_key must be configured together")
			}
		case "aliyun_oss":
			if strings.TrimSpace(c.FileStorage.AliyunOSS.Endpoint) == "" || strings.TrimSpace(c.FileStorage.AliyunOSS.Bucket) == "" || strings.TrimSpace(c.FileStorage.AliyunOSS.AccessKey) == "" || strings.TrimSpace(c.FileStorage.AliyunOSS.SecretKey) == "" {
				return fmt.Errorf("file_storage.aliyun_oss endpoint, bucket, access_key and secret_key are required")
			}
		default:
			return fmt.Errorf("file_storage.type must be local, s3 or aliyun_oss")
		}
	}
	if c.Server.ManagementPort <= 0 {
		return fmt.Errorf("server.management_port is required")
	}
	if c.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("server.shutdown_timeout must be positive")
	}
	if strings.TrimSpace(c.MySQL.DSN) == "" {
		return fmt.Errorf("mysql.dsn is required")
	}
	if role == "job" || role == "consumer" {
		if strings.TrimSpace(c.RocketMQ.Endpoints) == "" {
			return fmt.Errorf("rocketmq endpoints are required")
		}
		if (strings.TrimSpace(c.RocketMQ.AccessKey) == "") != (strings.TrimSpace(c.RocketMQ.SecretKey) == "") {
			return fmt.Errorf("rocketmq access_key and secret_key must be configured together")
		}
		if strings.TrimSpace(c.RocketMQ.TopicPrefix) == "" || len(c.RocketMQ.Topics) == 0 || c.RocketMQ.MaxBodyBytes <= 0 {
			return fmt.Errorf("rocketmq topic_prefix, topics, and max_body_bytes are required")
		}
	}
	if role == "consumer" {
		if strings.TrimSpace(c.RocketMQ.ConsumerGroup) == "" || c.RocketMQ.Concurrency <= 0 || c.RocketMQ.AwaitDuration <= 0 || c.RocketMQ.HandlerTimeout <= 0 {
			return fmt.Errorf("rocketmq consumer_group, concurrency, await_duration, and handler_timeout are required")
		}
	}
	return nil
}

func mapEnvironmentKey(key string) string {
	key = strings.TrimPrefix(key, "APP_")
	mapping := map[string]string{
		"ENVIRONMENT":                        "environment",
		"SERVER_HTTP_PORT":                   "server.http_port",
		"SERVER_MANAGEMENT_PORT":             "server.management_port",
		"SERVER_READ_HEADER_TIMEOUT":         "server.read_header_timeout",
		"SERVER_READ_TIMEOUT":                "server.read_timeout",
		"SERVER_WRITE_TIMEOUT":               "server.write_timeout",
		"SERVER_IDLE_TIMEOUT":                "server.idle_timeout",
		"SERVER_SHUTDOWN_TIMEOUT":            "server.shutdown_timeout",
		"SERVER_MAX_HEADER_BYTES":            "server.max_header_bytes",
		"SERVER_MAX_BODY_BYTES":              "server.max_body_bytes",
		"LOGGING_LEVEL":                      "logging.level",
		"LOGGING_FORMAT":                     "logging.format",
		"MYSQL_DSN":                          "mysql.dsn",
		"MYSQL_MAX_OPEN_CONNS":               "mysql.max_open_conns",
		"MYSQL_MAX_IDLE_CONNS":               "mysql.max_idle_conns",
		"REDIS_ADDRESS":                      "redis.address",
		"REDIS_PASSWORD":                     "redis.password",
		"REDIS_DATABASE":                     "redis.database",
		"IDENTITY_ACCESS_TOKEN_TTL":          "identity.access_token_ttl",
		"IDENTITY_REFRESH_TOKEN_TTL":         "identity.refresh_token_ttl",
		"IDENTITY_CAPTCHA_TTL":               "identity.captcha_ttl",
		"IDENTITY_BOOTSTRAP_USER":            "identity.bootstrap_user",
		"IDENTITY_BOOTSTRAP_PASSWORD":        "identity.bootstrap_password",
		"IDENTITY_DEFAULT_PASSWORD":          "identity.default_password",
		"FILE_STORAGE_ROOT":                  "file_storage.root",
		"FILE_STORAGE_TYPE":                  "file_storage.type",
		"FILE_STORAGE_PUBLIC_BASE_URL":       "file_storage.public_base_url",
		"FILE_STORAGE_MAX_FILE_BYTES":        "file_storage.max_file_bytes",
		"FILE_STORAGE_S3_ENDPOINT":           "file_storage.s3.endpoint",
		"FILE_STORAGE_S3_REGION":             "file_storage.s3.region",
		"FILE_STORAGE_S3_BUCKET":             "file_storage.s3.bucket",
		"FILE_STORAGE_S3_ACCESS_KEY":         "file_storage.s3.access_key",
		"FILE_STORAGE_S3_SECRET_KEY":         "file_storage.s3.secret_key",
		"FILE_STORAGE_S3_USE_PATH_STYLE":     "file_storage.s3.use_path_style",
		"FILE_STORAGE_ALIYUN_OSS_ENDPOINT":   "file_storage.aliyun_oss.endpoint",
		"FILE_STORAGE_ALIYUN_OSS_BUCKET":     "file_storage.aliyun_oss.bucket",
		"FILE_STORAGE_ALIYUN_OSS_ACCESS_KEY": "file_storage.aliyun_oss.access_key",
		"FILE_STORAGE_ALIYUN_OSS_SECRET_KEY": "file_storage.aliyun_oss.secret_key",
		"ROCKETMQ_ENDPOINTS":                 "rocketmq.endpoints",
		"ROCKETMQ_ACCESS_KEY":                "rocketmq.access_key",
		"ROCKETMQ_SECRET_KEY":                "rocketmq.secret_key",
		"ROCKETMQ_TOPIC_PREFIX":              "rocketmq.topic_prefix",
		"ROCKETMQ_CONSUMER_GROUP":            "rocketmq.consumer_group",
		"ROCKETMQ_HANDLER_TIMEOUT":           "rocketmq.handler_timeout",
	}
	if mapped, ok := mapping[key]; ok {
		return mapped
	}
	return "unused." + strings.ToLower(key)
}
