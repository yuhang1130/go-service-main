package auth

import (
	"context"
	"errors"
)

type Principal struct {
	Subject     string
	Name        string
	Permissions map[string]struct{}
	TenantID    string
	System      bool
}

func (p Principal) Has(permission string) bool {
	if p.System {
		return true
	}
	_, ok := p.Permissions[permission]
	return ok
}

type Verifier interface {
	VerifyAccessToken(ctx context.Context, token string) (Principal, error)
}

var ErrVerifierNotConfigured = errors.New("upstream token verifier is not configured")

type UnconfiguredVerifier struct{}

func (UnconfiguredVerifier) VerifyAccessToken(context.Context, string) (Principal, error) {
	return Principal{}, ErrVerifierNotConfigured
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}
