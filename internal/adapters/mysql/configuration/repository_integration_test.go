//go:build integration

package configuration

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	configurationdomain "github.com/yuhang1130/go-service-main/internal/features/configuration/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
)

func TestRepositoryAllowsRepeatedDeleteAndRecreate(t *testing.T) {
	dsn := os.Getenv("APP_MYSQL_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("APP_MYSQL_DSN is required in CI")
		}
		t.Skip("APP_MYSQL_DSN is not set")
	}
	ctx := context.Background()
	cfg := config.Defaults().MySQL
	cfg.DSN = dsn
	database, err := mysqladapter.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	repository := NewRepository(database.GORM())
	key := "integration.recreated." + uuid.NewString()
	t.Cleanup(func() { database.GORM().Exec("DELETE FROM sys_config WHERE config_key = ?", key) })
	for attempt := 0; attempt < 3; attempt++ {
		if err := repository.Create(ctx, configurationdomain.Config{Name: "Recreated", Key: key, Value: "value"}, 1); err != nil {
			t.Fatalf("create attempt %d: %v", attempt+1, err)
		}
		if attempt == 0 {
			err := repository.Create(ctx, configurationdomain.Config{Name: "Duplicate", Key: key, Value: "value"}, 1)
			if !errors.Is(err, persistence.ErrConflict) {
				t.Fatalf("duplicate create error = %v, want persistence conflict", err)
			}
		}
		item, err := repository.GetByKey(ctx, key)
		if err != nil {
			t.Fatalf("get attempt %d: %v", attempt+1, err)
		}
		if attempt < 2 {
			if err := repository.Delete(ctx, item.ID, 1); err != nil {
				t.Fatalf("delete attempt %d: %v", attempt+1, err)
			}
		}
	}
}
