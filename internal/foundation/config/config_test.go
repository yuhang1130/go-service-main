package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesEnvironmentAfterYAML(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "api.yaml")
	contents := []byte(`environment: local
server:
  http_port: 8080
  management_port: 9090
  shutdown_timeout: 10s
logging:
  level: info
  format: text
task_dispatch:
  stream_prefix: test
  stream: material:collection:v1
  consumer_group: test-collection
  read_block: 1s
  handler_timeout: 1m
  reclaim_interval: 15s
  claim_min_idle: 30s
  concurrency: 1
  max_message_bytes: 1024
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_SERVER_MANAGEMENT_PORT", "9191")
	t.Setenv("APP_MYSQL_DSN", "app:app@tcp(localhost:3306)/app?parseTime=true&loc=UTC")
	t.Setenv("APP_MYSQL_CONN_MAX_LIFETIME", "45m")
	t.Setenv("APP_MYSQL_CONN_MAX_IDLE_TIME", "7m")
	t.Setenv("APP_REDIS_DIAL_TIMEOUT", "4s")
	t.Setenv("APP_REDIS_READ_TIMEOUT", "3s")
	t.Setenv("APP_REDIS_WRITE_TIMEOUT", "5s")
	t.Setenv("APP_RESILIENCE_HTTP_RATE_LIMIT_ENABLED", "true")
	t.Setenv("APP_RESILIENCE_HTTP_RATE_LIMIT_RPS", "25")
	t.Setenv("APP_RESILIENCE_HTTP_RATE_LIMIT_BURST", "50")
	t.Setenv("APP_RESILIENCE_HTTP_RATE_LIMIT_CLIENT_TTL", "15m")
	t.Setenv("APP_RESILIENCE_CIRCUIT_BREAKER_FAILURE_THRESHOLD", "7")
	t.Setenv("APP_RESILIENCE_CIRCUIT_BREAKER_OPEN_TIMEOUT", "45s")
	t.Setenv("APP_RESILIENCE_CIRCUIT_BREAKER_HALF_OPEN_MAX", "2")
	t.Setenv("APP_IDENTITY_LOGIN_RATE_LIMIT", "15")
	t.Setenv("APP_IDENTITY_LOGIN_RATE_WINDOW", "2m")
	t.Setenv("APP_TASK_DISPATCH_READ_BLOCK", "6s")
	t.Setenv("APP_TASK_DISPATCH_HANDLER_TIMEOUT", "35m")
	t.Setenv("APP_TASK_DISPATCH_RECLAIM_INTERVAL", "20s")
	t.Setenv("APP_TASK_DISPATCH_CLAIM_MIN_IDLE", "45s")
	t.Setenv("APP_TASK_DISPATCH_CONCURRENCY", "12")
	t.Setenv("APP_TASK_DISPATCH_MAX_MESSAGE_BYTES", "8192")
	cfg := Defaults()
	if err := Load(path, "api", &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ManagementPort != 9191 {
		t.Fatalf("management port = %d, want 9191", cfg.Server.ManagementPort)
	}
	if cfg.MySQL.ConnMaxLifetime != 45*time.Minute || cfg.MySQL.ConnMaxIdleTime != 7*time.Minute {
		t.Fatalf("MySQL connection TTLs = %s/%s", cfg.MySQL.ConnMaxLifetime, cfg.MySQL.ConnMaxIdleTime)
	}
	if cfg.Redis.DialTimeout != 4*time.Second || cfg.Redis.ReadTimeout != 3*time.Second || cfg.Redis.WriteTimeout != 5*time.Second {
		t.Fatalf("Redis timeouts = %s/%s/%s", cfg.Redis.DialTimeout, cfg.Redis.ReadTimeout, cfg.Redis.WriteTimeout)
	}
	if cfg.Resilience.HTTPRateLimit.RequestsPerSecond != 25 || cfg.Resilience.HTTPRateLimit.Burst != 50 || cfg.Resilience.HTTPRateLimit.ClientTTL != 15*time.Minute {
		t.Fatalf("HTTP rate limit config = %#v", cfg.Resilience.HTTPRateLimit)
	}
	if cfg.Resilience.CircuitBreaker.FailureThreshold != 7 || cfg.Resilience.CircuitBreaker.OpenTimeout != 45*time.Second || cfg.Resilience.CircuitBreaker.HalfOpenMaxRequests != 2 {
		t.Fatalf("circuit breaker config = %#v", cfg.Resilience.CircuitBreaker)
	}
	if cfg.Identity.LoginRateLimit != 15 || cfg.Identity.LoginRateWindow != 2*time.Minute {
		t.Fatalf("identity login rate = %d/%s", cfg.Identity.LoginRateLimit, cfg.Identity.LoginRateWindow)
	}
	if cfg.TaskDispatch.ReadBlock != 6*time.Second ||
		cfg.TaskDispatch.HandlerTimeout != 35*time.Minute ||
		cfg.TaskDispatch.ReclaimInterval != 20*time.Second ||
		cfg.TaskDispatch.ClaimMinIdle != 45*time.Second {
		t.Fatalf("task dispatch durations = %#v", cfg.TaskDispatch)
	}
	if cfg.TaskDispatch.Concurrency != 12 || cfg.TaskDispatch.MaxMessageBytes != 8192 {
		t.Fatalf("task dispatch overrides = %#v", cfg.TaskDispatch)
	}
}

func TestValidateRequiresOnlyRoleCapabilities(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.MySQL.DSN = "app:app@tcp(localhost:3306)/app"
	cfg.Redis.Address = "localhost:6379"
	cfg.TaskDispatch = TaskDispatch{}

	if err := cfg.Validate("api"); err != nil {
		t.Fatalf("api should require MySQL and Redis but not task dispatch configuration: %v", err)
	}
	if err := cfg.Validate("job"); err == nil {
		t.Fatal("job should require task dispatch publisher configuration")
	}
}

func TestValidateConsumerRequiresConsumerSettings(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.MySQL.DSN = "app:app@tcp(localhost:3306)/app"
	cfg.Redis.Address = "localhost:6379"
	cfg.TaskDispatch = TaskDispatch{StreamPrefix: "test", MaxMessageBytes: 1024}

	if err := cfg.Validate("collection-consumer"); err == nil {
		t.Fatal("consumer should require stream, group, concurrency, and durations")
	}
}

func TestRepositoryRoleConfigsValidate(t *testing.T) {
	for _, role := range []string{
		"api",
		"job",
		"collection-consumer",
		"transformation-consumer",
		"upload-consumer",
		"infrastructure-consumer",
	} {
		role := role
		t.Run(role, func(t *testing.T) {
			t.Setenv("APP_MYSQL_DSN", "app:app@tcp(localhost:3306)/app?parseTime=true&loc=UTC")
			t.Setenv("APP_REDIS_ADDRESS", "localhost:6379")
			cfg := Defaults()
			path := filepath.Join("..", "..", "..", "configs", role+".yaml")
			if err := Load(path, role, &cfg); err != nil {
				t.Fatalf("load %s config: %v", role, err)
			}
		})
	}
}
