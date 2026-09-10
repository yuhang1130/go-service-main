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
	LoginRateLimit  int           `koanf:"login_rate_limit"`
	LoginRateWindow time.Duration `koanf:"login_rate_window"`
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
type TaskDispatch struct {
	StreamPrefix    string        `koanf:"stream_prefix"`
	Stream          string        `koanf:"stream"`
	ConsumerGroup   string        `koanf:"consumer_group"`
	ConsumerName    string        `koanf:"consumer_name"`
	ReadBlock       time.Duration `koanf:"read_block"`
	HandlerTimeout  time.Duration `koanf:"handler_timeout"`
	ReclaimInterval time.Duration `koanf:"reclaim_interval"`
	ClaimMinIdle    time.Duration `koanf:"claim_min_idle"`
	Concurrency     int           `koanf:"concurrency"`
	MaxMessageBytes int           `koanf:"max_message_bytes"`
}

type HTTPRateLimit struct {
	Enabled           bool          `koanf:"enabled"`
	RequestsPerSecond float64       `koanf:"requests_per_second"`
	Burst             int           `koanf:"burst"`
	ClientTTL         time.Duration `koanf:"client_ttl"`
}

type CircuitBreaker struct {
	FailureThreshold    uint32        `koanf:"failure_threshold"`
	OpenTimeout         time.Duration `koanf:"open_timeout"`
	HalfOpenMaxRequests uint32        `koanf:"half_open_max_requests"`
}

type Resilience struct {
	HTTPRateLimit  HTTPRateLimit  `koanf:"http_rate_limit"`
	CircuitBreaker CircuitBreaker `koanf:"circuit_breaker"`
}

type Role struct {
	Environment  string         `koanf:"environment"`
	Server       Server         `koanf:"server"`
	Logging      logging.Config `koanf:"logging"`
	MySQL        MySQL          `koanf:"mysql"`
	Redis        Redis          `koanf:"redis"`
	Identity     Identity       `koanf:"identity"`
	FileStorage  FileStorage    `koanf:"file_storage"`
	TaskDispatch TaskDispatch   `koanf:"task_dispatch"`
	Resilience   Resilience     `koanf:"resilience"`
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
		Logging: logging.Config{Level: "info", Format: "text"},
		MySQL:   MySQL{MaxOpenConns: 25, MaxIdleConns: 10, ConnMaxLifetime: 30 * time.Minute, ConnMaxIdleTime: 5 * time.Minute},
		Redis:   Redis{Address: "127.0.0.1:6379", DialTimeout: 3 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second},
		Identity: Identity{
			AccessTokenTTL: 2 * time.Hour, RefreshTokenTTL: 7 * 24 * time.Hour, CaptchaTTL: 5 * time.Minute,
			LoginRateLimit: 10, LoginRateWindow: time.Minute,
		},
		FileStorage: FileStorage{Type: "local", Root: ".tmp/uploads", MaxFileBytes: 2 << 20, S3: S3Storage{Region: "us-east-1", UsePathStyle: true}},
		TaskDispatch: TaskDispatch{
			StreamPrefix: "go-service-main", ReadBlock: 5 * time.Second,
			HandlerTimeout: 2 * time.Hour, ReclaimInterval: 30 * time.Second,
			ClaimMinIdle: time.Minute, Concurrency: 1, MaxMessageBytes: 4096,
		},
		Resilience: Resilience{
			HTTPRateLimit: HTTPRateLimit{RequestsPerSecond: 100, Burst: 200, ClientTTL: 10 * time.Minute},
			CircuitBreaker: CircuitBreaker{
				FailureThreshold: 5, OpenTimeout: 30 * time.Second, HalfOpenMaxRequests: 1,
			},
		},
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
	if role != "api" && role != "job" && !isTaskConsumerRole(role) {
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
		if c.Identity.LoginRateLimit <= 0 || c.Identity.LoginRateWindow <= 0 {
			return fmt.Errorf("identity login rate limit and window must be positive")
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
	if role == "api" && c.Resilience.HTTPRateLimit.Enabled &&
		(c.Resilience.HTTPRateLimit.RequestsPerSecond <= 0 || c.Resilience.HTTPRateLimit.Burst <= 0 || c.Resilience.HTTPRateLimit.ClientTTL <= 0) {
		return fmt.Errorf("resilience.http_rate_limit settings must be positive when enabled")
	}
	if role == "job" && (c.Resilience.CircuitBreaker.FailureThreshold == 0 ||
		c.Resilience.CircuitBreaker.OpenTimeout <= 0 ||
		c.Resilience.CircuitBreaker.HalfOpenMaxRequests == 0) {
		return fmt.Errorf("resilience.circuit_breaker settings must be positive")
	}
	if strings.TrimSpace(c.MySQL.DSN) == "" {
		return fmt.Errorf("mysql.dsn is required")
	}
	if role == "job" || isTaskConsumerRole(role) {
		if strings.TrimSpace(c.Redis.Address) == "" {
			return fmt.Errorf("redis.address is required for task dispatch")
		}
		if strings.TrimSpace(c.TaskDispatch.StreamPrefix) == "" || c.TaskDispatch.MaxMessageBytes <= 0 {
			return fmt.Errorf("task_dispatch stream_prefix and max_message_bytes are required")
		}
	}
	if isTaskConsumerRole(role) {
		if strings.TrimSpace(c.TaskDispatch.Stream) == "" ||
			strings.TrimSpace(c.TaskDispatch.ConsumerGroup) == "" ||
			c.TaskDispatch.Concurrency <= 0 ||
			c.TaskDispatch.ReadBlock <= 0 ||
			c.TaskDispatch.HandlerTimeout <= 0 ||
			c.TaskDispatch.ReclaimInterval <= 0 ||
			c.TaskDispatch.ClaimMinIdle <= 0 {
			return fmt.Errorf("task_dispatch consumer settings are required")
		}
	}
	return nil
}

func isTaskConsumerRole(role string) bool {
	switch role {
	case "collection-consumer", "transformation-consumer", "upload-consumer", "infrastructure-consumer":
		return true
	default:
		return false
	}
}

func mapEnvironmentKey(key string) string {
	key = strings.TrimPrefix(key, "APP_")
	mapping := map[string]string{
		"ENVIRONMENT":                                  "environment",
		"SERVER_HTTP_PORT":                             "server.http_port",
		"SERVER_MANAGEMENT_PORT":                       "server.management_port",
		"SERVER_READ_HEADER_TIMEOUT":                   "server.read_header_timeout",
		"SERVER_READ_TIMEOUT":                          "server.read_timeout",
		"SERVER_WRITE_TIMEOUT":                         "server.write_timeout",
		"SERVER_IDLE_TIMEOUT":                          "server.idle_timeout",
		"SERVER_SHUTDOWN_TIMEOUT":                      "server.shutdown_timeout",
		"SERVER_MAX_HEADER_BYTES":                      "server.max_header_bytes",
		"SERVER_MAX_BODY_BYTES":                        "server.max_body_bytes",
		"LOGGING_LEVEL":                                "logging.level",
		"LOGGING_FORMAT":                               "logging.format",
		"MYSQL_DSN":                                    "mysql.dsn",
		"MYSQL_MAX_OPEN_CONNS":                         "mysql.max_open_conns",
		"MYSQL_MAX_IDLE_CONNS":                         "mysql.max_idle_conns",
		"MYSQL_CONN_MAX_LIFETIME":                      "mysql.conn_max_lifetime",
		"MYSQL_CONN_MAX_IDLE_TIME":                     "mysql.conn_max_idle_time",
		"REDIS_ADDRESS":                                "redis.address",
		"REDIS_PASSWORD":                               "redis.password",
		"REDIS_DATABASE":                               "redis.database",
		"REDIS_DIAL_TIMEOUT":                           "redis.dial_timeout",
		"REDIS_READ_TIMEOUT":                           "redis.read_timeout",
		"REDIS_WRITE_TIMEOUT":                          "redis.write_timeout",
		"IDENTITY_ACCESS_TOKEN_TTL":                    "identity.access_token_ttl",
		"IDENTITY_REFRESH_TOKEN_TTL":                   "identity.refresh_token_ttl",
		"IDENTITY_CAPTCHA_TTL":                         "identity.captcha_ttl",
		"IDENTITY_LOGIN_RATE_LIMIT":                    "identity.login_rate_limit",
		"IDENTITY_LOGIN_RATE_WINDOW":                   "identity.login_rate_window",
		"IDENTITY_BOOTSTRAP_USER":                      "identity.bootstrap_user",
		"IDENTITY_BOOTSTRAP_PASSWORD":                  "identity.bootstrap_password",
		"IDENTITY_DEFAULT_PASSWORD":                    "identity.default_password",
		"FILE_STORAGE_ROOT":                            "file_storage.root",
		"FILE_STORAGE_TYPE":                            "file_storage.type",
		"FILE_STORAGE_PUBLIC_BASE_URL":                 "file_storage.public_base_url",
		"FILE_STORAGE_MAX_FILE_BYTES":                  "file_storage.max_file_bytes",
		"FILE_STORAGE_S3_ENDPOINT":                     "file_storage.s3.endpoint",
		"FILE_STORAGE_S3_REGION":                       "file_storage.s3.region",
		"FILE_STORAGE_S3_BUCKET":                       "file_storage.s3.bucket",
		"FILE_STORAGE_S3_ACCESS_KEY":                   "file_storage.s3.access_key",
		"FILE_STORAGE_S3_SECRET_KEY":                   "file_storage.s3.secret_key",
		"FILE_STORAGE_S3_USE_PATH_STYLE":               "file_storage.s3.use_path_style",
		"FILE_STORAGE_ALIYUN_OSS_ENDPOINT":             "file_storage.aliyun_oss.endpoint",
		"FILE_STORAGE_ALIYUN_OSS_BUCKET":               "file_storage.aliyun_oss.bucket",
		"FILE_STORAGE_ALIYUN_OSS_ACCESS_KEY":           "file_storage.aliyun_oss.access_key",
		"FILE_STORAGE_ALIYUN_OSS_SECRET_KEY":           "file_storage.aliyun_oss.secret_key",
		"TASK_DISPATCH_STREAM_PREFIX":                  "task_dispatch.stream_prefix",
		"TASK_DISPATCH_STREAM":                         "task_dispatch.stream",
		"TASK_DISPATCH_CONSUMER_GROUP":                 "task_dispatch.consumer_group",
		"TASK_DISPATCH_CONSUMER_NAME":                  "task_dispatch.consumer_name",
		"TASK_DISPATCH_READ_BLOCK":                     "task_dispatch.read_block",
		"TASK_DISPATCH_HANDLER_TIMEOUT":                "task_dispatch.handler_timeout",
		"TASK_DISPATCH_RECLAIM_INTERVAL":               "task_dispatch.reclaim_interval",
		"TASK_DISPATCH_CLAIM_MIN_IDLE":                 "task_dispatch.claim_min_idle",
		"TASK_DISPATCH_CONCURRENCY":                    "task_dispatch.concurrency",
		"TASK_DISPATCH_MAX_MESSAGE_BYTES":              "task_dispatch.max_message_bytes",
		"RESILIENCE_HTTP_RATE_LIMIT_ENABLED":           "resilience.http_rate_limit.enabled",
		"RESILIENCE_HTTP_RATE_LIMIT_RPS":               "resilience.http_rate_limit.requests_per_second",
		"RESILIENCE_HTTP_RATE_LIMIT_BURST":             "resilience.http_rate_limit.burst",
		"RESILIENCE_HTTP_RATE_LIMIT_CLIENT_TTL":        "resilience.http_rate_limit.client_ttl",
		"RESILIENCE_CIRCUIT_BREAKER_FAILURE_THRESHOLD": "resilience.circuit_breaker.failure_threshold",
		"RESILIENCE_CIRCUIT_BREAKER_OPEN_TIMEOUT":      "resilience.circuit_breaker.open_timeout",
		"RESILIENCE_CIRCUIT_BREAKER_HALF_OPEN_MAX":     "resilience.circuit_breaker.half_open_max_requests",
	}
	if mapped, ok := mapping[key]; ok {
		return mapped
	}
	return "unused." + strings.ToLower(key)
}
