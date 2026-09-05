package local

import (
	"context"
	"strconv"

	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

type Verifier struct{ identities *identityapp.Service }

func NewVerifier(identities *identityapp.Service) *Verifier { return &Verifier{identities: identities} }

func (v *Verifier) VerifyAccessToken(ctx context.Context, token string) (auth.Principal, error) {
	account, authorization, err := v.identities.VerifyAccount(ctx, token)
	if err != nil {
		return auth.Principal{}, err
	}
	permissions := make(map[string]struct{}, len(authorization.Permissions))
	for _, permission := range authorization.Permissions {
		permissions[permission] = struct{}{}
	}
	return auth.Principal{Subject: strconv.FormatInt(account.ID, 10), Name: account.Nickname, Permissions: permissions, System: authorization.System}, nil
}
