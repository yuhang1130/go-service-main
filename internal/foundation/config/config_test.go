package config

import (
	"os"
	"path/filepath"
	"reflect"
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
rocketmq:
  endpoints: 127.0.0.1:8081
  access_key: test
  secret_key: test
  topic_prefix: test
  consumer_group: test
  await_duration: 1s
  topics: [events]
  concurrency: 1
  max_body_bytes: 1024
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
	t.Setenv("APP_IDENTITY_LOGIN_RATE_LIMIT", "15")
	t.Setenv("APP_IDENTITY_LOGIN_RATE_WINDOW", "2m")
	t.Setenv("APP_ROCKETMQ_AWAIT_DURATION", "6s")
	t.Setenv("APP_ROCKETMQ_HANDLER_TIMEOUT", "35s")
	t.Setenv("APP_ROCKETMQ_TOPICS", "events,audit")
	t.Setenv("APP_ROCKETMQ_CONCURRENCY", "12")
	t.Setenv("APP_ROCKETMQ_MAX_BODY_BYTES", "2097152")
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
	if cfg.Identity.LoginRateLimit != 15 || cfg.Identity.LoginRateWindow != 2*time.Minute {
		t.Fatalf("identity login rate = %d/%s", cfg.Identity.LoginRateLimit, cfg.Identity.LoginRateWindow)
	}
	if cfg.RocketMQ.AwaitDuration != 6*time.Second || cfg.RocketMQ.HandlerTimeout != 35*time.Second {
		t.Fatalf("RocketMQ durations = %s/%s", cfg.RocketMQ.AwaitDuration, cfg.RocketMQ.HandlerTimeout)
	}
	if !reflect.DeepEqual(cfg.RocketMQ.Topics, []string{"events", "audit"}) || cfg.RocketMQ.Concurrency != 12 || cfg.RocketMQ.MaxBodyBytes != 2097152 {
		t.Fatalf("RocketMQ overrides = topics %v, concurrency %d, max body %d", cfg.RocketMQ.Topics, cfg.RocketMQ.Concurrency, cfg.RocketMQ.MaxBodyBytes)
	}
}

func TestValidateRequiresOnlyRoleCapabilities(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.MySQL.DSN = "app:app@tcp(localhost:3306)/app"
	cfg.Redis.Address = "localhost:6379"
	cfg.RocketMQ = RocketMQ{}

	if err := cfg.Validate("api"); err != nil {
		t.Fatalf("api should require MySQL and Redis but not RocketMQ configuration: %v", err)
	}
	if err := cfg.Validate("job"); err == nil {
		t.Fatal("job should require RocketMQ producer configuration")
	}
}

func TestValidateConsumerRequiresConsumerSettings(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.MySQL.DSN = "app:app@tcp(localhost:3306)/app"
	cfg.RocketMQ = RocketMQ{
		Endpoints:   "localhost:8081",
		TopicPrefix: "test", Topics: []string{"events"}, MaxBodyBytes: 1024,
	}

	if err := cfg.Validate("consumer"); err == nil {
		t.Fatal("consumer should require consumer_group, concurrency, and await_duration")
	}
}

func TestRepositoryRoleConfigsValidate(t *testing.T) {
	for _, role := range []string{"api", "job", "consumer"} {
		role := role
		t.Run(role, func(t *testing.T) {
			cfg := Defaults()
			path := filepath.Join("..", "..", "..", "configs", role+".yaml")
			if err := Load(path, role, &cfg); err != nil {
				t.Fatalf("load %s config: %v", role, err)
			}
		})
	}
}
