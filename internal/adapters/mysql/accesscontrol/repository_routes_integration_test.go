//go:build integration

package accesscontrol

import (
	"context"
	"os"
	"testing"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func TestMenusForAccountIncludesHiddenMenus(t *testing.T) {
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

	transaction := database.GORM().Begin()
	if transaction.Error != nil {
		t.Fatal(transaction.Error)
	}
	t.Cleanup(func() { _ = transaction.Rollback().Error })

	now := time.Now().UTC()
	hidden := menuRow{
		ID:         9_000_000_001,
		ParentID:   0,
		TreePath:   "0",
		Name:       "Hidden integration route",
		Type:       "M",
		RouteName:  "HiddenIntegrationRoute",
		RoutePath:  "hidden-integration-route",
		Component:  "error/404",
		Visible:    0,
		Sort:       1,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := transaction.Create(&hidden).Error; err != nil {
		t.Fatal(err)
	}

	items, err := NewRepository(transaction).MenusForAccount(ctx, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == hidden.ID && item.RouteName == hidden.RouteName && item.Visible == 0 {
			return
		}
	}
	t.Fatalf("hidden menu %d was not returned", hidden.ID)
}
