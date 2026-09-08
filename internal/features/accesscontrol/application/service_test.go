package application

import (
	"context"
	"net/http"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type repositoryStub struct {
	Repository
	writeErr error
	deleted  []int64
}

func (repositoryStub) RoleCodeExists(context.Context, string, int64) (bool, error) {
	return false, nil
}

func (repositoryStub) RoleNameExists(context.Context, string, int64) (bool, error) {
	return false, nil
}

func (r repositoryStub) SaveRole(context.Context, domain.Role, int64) error {
	return r.writeErr
}

func (r repositoryStub) SetRoleMenus(context.Context, int64, []int64, int64) error {
	return r.writeErr
}

func (r repositoryStub) SetRoleDepartments(context.Context, int64, []int64, int64) error {
	return r.writeErr
}

func (r *repositoryStub) DeleteRoles(_ context.Context, ids []int64, _ int64) error {
	r.deleted = append([]int64(nil), ids...)
	return r.writeErr
}

func TestSaveRoleMapsInvalidAssociationsToBadRequest(t *testing.T) {
	t.Parallel()
	service := NewService(&repositoryStub{writeErr: ErrInvalidAssociation}, nil)

	err := service.SaveRole(context.Background(), RoleCommand{Name: "运营", Code: "OPS", Status: 1, DataScope: domain.ScopeSelf, MenuIDs: []int64{999}}, 1)
	assertBadRequest(t, err)
}

func TestSetRoleAssociationsMapInvalidTargetsToBadRequest(t *testing.T) {
	t.Parallel()
	service := NewService(&repositoryStub{writeErr: ErrInvalidAssociation}, nil)

	assertBadRequest(t, service.SetRoleMenus(context.Background(), 2, []int64{999}, 1))
	assertBadRequest(t, service.SetRoleDepartments(context.Background(), 2, []int64{999}, 1))
}

func TestDeleteRolesUsesOneAtomicRepositoryCall(t *testing.T) {
	t.Parallel()
	repository := &repositoryStub{}
	service := NewService(repository, nil)

	if err := service.DeleteRoles(context.Background(), []int64{3, 2, 3}, 1); err != nil {
		t.Fatal(err)
	}
	if len(repository.deleted) != 2 || repository.deleted[0] != 3 || repository.deleted[1] != 2 {
		t.Fatalf("deleted IDs = %v, want [3 2]", repository.deleted)
	}
}

func TestDeleteRolesMapsRepositoryGuards(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{ErrProtectedRole, http.StatusForbidden, apperror.CodeForbidden},
		{ErrRoleInUse, http.StatusConflict, apperror.CodeConflict},
		{ErrNotFound, http.StatusNotFound, apperror.CodeNotFound},
	}
	for _, test := range tests {
		repository := &repositoryStub{writeErr: test.err}
		err := NewService(repository, nil).DeleteRoles(context.Background(), []int64{2}, 1)
		applicationError := apperror.As(err)
		if applicationError.HTTPStatus != test.status || applicationError.Code != test.code {
			t.Fatalf("error = %#v, want HTTP %d/%s", applicationError, test.status, test.code)
		}
	}
}

func assertBadRequest(t *testing.T, err error) {
	t.Helper()
	applicationError := apperror.As(err)
	if applicationError.HTTPStatus != http.StatusBadRequest || applicationError.Code != apperror.CodeInvalidArgument {
		t.Fatalf("error = %#v, want HTTP 400/%s", applicationError, apperror.CodeInvalidArgument)
	}
}
