package bootstrap

import (
	"context"

	redisclient "github.com/redis/go-redis/v9"
	localauth "github.com/yuhang1130/go-service-main/internal/adapters/auth/local"
	passwordauth "github.com/yuhang1130/go-service-main/internal/adapters/auth/password"
	accesshttp "github.com/yuhang1130/go-service-main/internal/adapters/http/accesscontrol"
	identityhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/identity"
	organizationhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/organization"
	accessmysql "github.com/yuhang1130/go-service-main/internal/adapters/mysql/accesscontrol"
	identitymysql "github.com/yuhang1130/go-service-main/internal/adapters/mysql/identity"
	organizationmysql "github.com/yuhang1130/go-service-main/internal/adapters/mysql/organization"
	identityredis "github.com/yuhang1130/go-service-main/internal/adapters/redis/identity"
	accessapp "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/application"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	organizationapp "github.com/yuhang1130/go-service-main/internal/features/organization/application"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"gorm.io/gorm"
)

type identityAccessAPI struct {
	identity     *identityapp.Service
	identityHTTP *identityhttp.Handler
	accessHTTP   *accesshttp.Handler
	organization *organizationhttp.Handler
	verifier     *localauth.Verifier
}

func wireIdentityAccessAPI(database *gorm.DB, redis *redisclient.Client, cfg config.Identity) identityAccessAPI {
	sessions := identityredis.NewStore(redis, cfg)
	access := accessapp.NewService(accessmysql.NewRepository(database), sessions)
	identities := identityapp.NewService(identitymysql.NewRepository(database), sessions, sessions, passwordauth.NewBcrypt(), identityAuthorizer{access}, cfg.DefaultPassword)
	organizations := organizationapp.NewService(organizationmysql.NewRepository(database))
	return identityAccessAPI{
		identity:     identities,
		identityHTTP: identityhttp.NewHandler(identities, sessions),
		accessHTTP:   accesshttp.NewHandler(access),
		organization: organizationhttp.NewHandler(organizations),
		verifier:     localauth.NewVerifier(identities),
	}
}

type identityAuthorizer struct {
	access *accessapp.Service
}

func (a identityAuthorizer) Authorization(ctx context.Context, accountID int64) (identityapp.Authorization, error) {
	authorization, err := a.access.Authorization(ctx, accountID)
	if err != nil {
		return identityapp.Authorization{}, err
	}
	return identityapp.Authorization{
		Roles:       authorization.Roles,
		Permissions: authorization.Permissions,
		System:      authorization.System,
	}, nil
}

func (a identityAuthorizer) Scope(ctx context.Context, accountID int64) (identityapp.AccountScope, error) {
	scope, err := a.access.Scope(ctx, accountID)
	if err != nil {
		return identityapp.AccountScope{}, err
	}
	return identityapp.AccountScope{
		All:           scope.All,
		SelfID:        scope.SelfID,
		DepartmentIDs: scope.DepartmentIDs,
	}, nil
}

var _ identityapp.Authorizer = identityAuthorizer{}
