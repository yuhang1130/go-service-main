//go:build integration

package accesscontrol

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/features/accesscontrol/application"
	"github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func TestRepositoryValidatesRoleAssociationsBeforeReplacement(t *testing.T) {
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

	suffix := uuid.NewString()[:8]
	invalidCode := "INVALID_" + suffix
	invalidRole := domain.Role{Name: "Invalid " + suffix, Code: invalidCode, Status: 1, DataScope: domain.ScopeSelf, MenuIDs: []int64{9223372036854775000}}
	if err := repository.SaveRole(ctx, invalidRole, 0); !errors.Is(err, application.ErrInvalidAssociation) {
		t.Fatalf("invalid role associations returned %v", err)
	}
	if exists, err := repository.RoleCodeExists(ctx, invalidCode, 0); err != nil || exists {
		t.Fatalf("invalid role must not be persisted: exists %v, error %v", exists, err)
	}

	code := "INTEGRATION_" + suffix
	role := domain.Role{Name: "Integration " + suffix, Code: code, Status: 1, DataScope: domain.ScopeSelf}
	if err := repository.SaveRole(ctx, role, 0); err != nil {
		t.Fatal(err)
	}
	var roleIDs []int64
	if err := database.GORM().Table("sys_role").Where("code = ? AND is_deleted = 0", code).Pluck("id", &roleIDs).Error; err != nil || len(roleIDs) != 1 {
		t.Fatalf("created role IDs = %v, error %v", roleIDs, err)
	}
	roleID := roleIDs[0]
	t.Cleanup(func() {
		database.GORM().Exec("DELETE FROM sys_role_menu WHERE role_id = ?", roleID)
		database.GORM().Exec("DELETE FROM sys_role_dept WHERE role_id = ?", roleID)
		database.GORM().Exec("DELETE FROM sys_role WHERE id = ?", roleID)
	})

	if err := repository.SetRoleMenus(ctx, roleID, []int64{1}, 0); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetRoleMenus(ctx, roleID, []int64{9223372036854775000}, 0); !errors.Is(err, application.ErrInvalidAssociation) {
		t.Fatalf("invalid menu association returned %v", err)
	}
	menuIDs, err := repository.RoleMenuIDs(ctx, roleID)
	assertIDs(t, menuIDs, err, []int64{1})

	if err := repository.SetRoleDepartments(ctx, roleID, []int64{1}, 0); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetRoleDepartments(ctx, roleID, []int64{9223372036854775000}, 0); !errors.Is(err, application.ErrInvalidAssociation) {
		t.Fatalf("invalid department association returned %v", err)
	}
	departmentIDs, err := repository.RoleDepartmentIDs(ctx, roleID)
	assertIDs(t, departmentIDs, err, []int64{1})

	if err := repository.DeleteRoles(ctx, []int64{roleID, 1}, 0); !errors.Is(err, application.ErrProtectedRole) {
		t.Fatalf("DeleteRoles() protected-role error = %v", err)
	}
	if exists, err := repository.RoleCodeExists(ctx, code, 0); err != nil || !exists {
		t.Fatalf("atomic protected-role rejection removed ordinary role: exists %v, error %v", exists, err)
	}
	if err := repository.DeleteRoles(ctx, []int64{roleID}, 0); err != nil {
		t.Fatalf("DeleteRoles() error = %v", err)
	}
	if exists, err := repository.RoleCodeExists(ctx, code, 0); err != nil || exists {
		t.Fatalf("soft-deleted role remains active: exists %v, error %v", exists, err)
	}
}

func assertIDs(t *testing.T, ids []int64, err error, want []int64) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(want) {
		t.Fatalf("IDs = %v, want %v", ids, want)
	}
	for index := range ids {
		if ids[index] != want[index] {
			t.Fatalf("IDs = %v, want %v", ids, want)
		}
	}
}
