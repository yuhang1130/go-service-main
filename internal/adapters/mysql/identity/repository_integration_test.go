//go:build integration

package identity

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	accessdomain "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	identitydomain "github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func TestRepositoryRoundTripsAccountAndRoles(t *testing.T) {
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

	username := "identity_" + uuid.NewString()
	repository := NewRepository(database.GORM())
	account := identitydomain.Account{Username: username, Nickname: "Integration", Gender: 0, Password: "test-only-hash", DepartmentID: 1, Status: 1}
	if err := repository.Save(ctx, account, []int64{1}, 0); err != nil {
		t.Fatal(err)
	}
	created, err := repository.GetByUsername(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var ids []int64
		database.GORM().Table("sys_user").Where("username = ?", username).Pluck("id", &ids)
		if len(ids) > 0 {
			database.GORM().Exec("DELETE FROM sys_user_role WHERE user_id IN ?", ids)
			database.GORM().Exec("DELETE FROM sys_user WHERE id IN ?", ids)
		}
	})

	got, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != username || len(got.RoleIDs) != 1 || got.RoleIDs[0] != 1 {
		t.Fatalf("unexpected account: %#v", got)
	}
	items, total, err := repository.List(ctx, identityapp.ListQuery{Page: 1, PageSize: 10, Keywords: username}, accessdomain.AccountScope{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Username != username {
		t.Fatalf("unexpected page: total=%d items=%#v", total, items)
	}
	if err := repository.Delete(ctx, []int64{created.ID}, 0); err != nil {
		t.Fatal(err)
	}
	exists, err := repository.UsernameExists(ctx, username, 0)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("UsernameExists() = true after soft delete, want false")
	}
	if err := repository.Save(ctx, account, []int64{1}, 0); err != nil {
		t.Fatalf("recreate soft-deleted username: %v", err)
	}
	recreated, err := repository.GetByUsername(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	if recreated.ID == created.ID {
		t.Fatalf("recreated account ID = %d, want a new row", recreated.ID)
	}
}
