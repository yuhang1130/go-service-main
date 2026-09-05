package application

import (
	"context"
	"errors"
	"net/http"
	"testing"

	accessdomain "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	identitydomain "github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
)

type repositoryStub struct {
	Repository
	account  identitydomain.Account
	root     bool
	count    int64
	created  bool
	writeErr error
}

func (r repositoryStub) GetByUsername(context.Context, string) (identitydomain.Account, error) {
	return r.account, nil
}

func (r repositoryStub) Get(context.Context, int64) (identitydomain.Account, error) {
	return r.account, nil
}

func (r repositoryStub) IsRoot(context.Context, int64) (bool, error) { return r.root, nil }

func (r repositoryStub) Count(context.Context) (int64, error) { return r.count, nil }

func (r repositoryStub) Bootstrap(context.Context, identitydomain.Account, string) (bool, error) {
	return r.created, r.writeErr
}

func (repositoryStub) UsernameExists(context.Context, string, int64) (bool, error) { return false, nil }

func (r repositoryStub) Save(context.Context, identitydomain.Account, []int64, int64) error {
	return r.writeErr
}

type sessionsStub struct {
	Sessions
	tokens    identitydomain.TokenPair
	accountID int64
}

func (s sessionsStub) Create(context.Context, int64) (identitydomain.TokenPair, error) {
	return s.tokens, nil
}

func (s sessionsStub) AccountID(context.Context, string) (int64, error) { return s.accountID, nil }

type captchasStub struct {
	Captchas
	valid bool
}

func (c captchasStub) Verify(context.Context, string, string) (bool, error) { return c.valid, nil }

type passwordsStub struct{ PasswordHasher }

func (passwordsStub) Compare(hash, value string) bool { return hash == "hash" && value == "password" }
func (passwordsStub) Hash(string) (string, error)     { return "hash", nil }

type authorizerStub struct{ Authorizer }

func (authorizerStub) Authorization(context.Context, int64) (accessdomain.Authorization, error) {
	return accessdomain.Authorization{}, nil
}

func TestLoginReturnsOpaqueSessionPair(t *testing.T) {
	want := identitydomain.TokenPair{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 7200}
	service := NewService(
		repositoryStub{account: identitydomain.Account{ID: 7, Password: "hash", Status: 1}},
		sessionsStub{tokens: want}, captchasStub{valid: true}, passwordsStub{}, authorizerStub{}, "",
	)
	got, err := service.Login(context.Background(), LoginCommand{Username: "admin", Password: "password", CaptchaID: "captcha", CaptchaCode: "2468"})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
}

func TestLoginConsumesCaptchaBeforeCredentials(t *testing.T) {
	service := NewService(repositoryStub{}, sessionsStub{}, captchasStub{valid: false}, passwordsStub{}, authorizerStub{}, "")
	_, err := service.Login(context.Background(), LoginCommand{Username: "admin", Password: "password", CaptchaID: "captcha", CaptchaCode: "wrong"})
	applicationError := apperror.As(err)
	if applicationError.Code != "A0400" {
		t.Fatalf("code = %q, want A0400", applicationError.Code)
	}
}

func TestDeleteRejectsRootAccount(t *testing.T) {
	service := NewService(repositoryStub{root: true}, sessionsStub{}, captchasStub{}, passwordsStub{}, authorizerStub{}, "")
	err := service.Delete(context.Background(), []int64{1}, 2)
	applicationError := apperror.As(err)
	if applicationError.Code != "A0300" {
		t.Fatalf("code = %q, want A0300", applicationError.Code)
	}
}

func TestBootstrapReportsWhetherAdministratorWasCreated(t *testing.T) {
	t.Parallel()
	service := NewService(repositoryStub{created: true}, sessionsStub{}, captchasStub{}, passwordsStub{}, authorizerStub{}, "")

	created, err := service.Bootstrap(context.Background(), "admin", "password123")
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if !created {
		t.Fatal("Bootstrap() created = false, want true")
	}
}

func TestBootstrapSkipsWhenAnActiveAccountExists(t *testing.T) {
	t.Parallel()
	service := NewService(repositoryStub{count: 1, created: true}, sessionsStub{}, captchasStub{}, passwordsStub{}, authorizerStub{}, "")

	created, err := service.Bootstrap(context.Background(), "admin", "password123")
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if created {
		t.Fatal("Bootstrap() created = true, want false")
	}
}

func TestVerifyAccountReturnsIdentitySnapshot(t *testing.T) {
	t.Parallel()
	want := identitydomain.Account{ID: 7, Username: "admin", Nickname: "管理员", Status: 1}
	service := NewService(repositoryStub{account: want}, sessionsStub{accountID: want.ID}, captchasStub{}, passwordsStub{}, authorizerStub{}, "")

	account, _, err := service.VerifyAccount(context.Background(), "access")
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != want.ID || account.Nickname != want.Nickname {
		t.Fatalf("account = %#v, want %#v", account, want)
	}
}

func TestSaveMapsLateUniqueConstraintFailureToConflict(t *testing.T) {
	t.Parallel()
	service := NewService(
		repositoryStub{writeErr: persistence.Conflict(errors.New("duplicate username"))},
		sessionsStub{}, captchasStub{}, passwordsStub{}, authorizerStub{}, "password123",
	)

	err := service.Save(context.Background(), SaveCommand{Username: "admin", Nickname: "管理员", Gender: 0, Status: 1, RoleIDs: []int64{1}}, 1)
	applicationError := apperror.As(err)
	if applicationError.HTTPStatus != http.StatusConflict || applicationError.Code != "A0409" {
		t.Fatalf("Save() error = %#v, want HTTP 409/A0409", applicationError)
	}
}
