package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

func Authenticate(verifier auth.Verifier) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			adminapi.Error(ctx, apperror.Unauthorized(apperror.CodeInvalidAccessToken, "访问令牌无效或已过期"))
			ctx.Abort()
			return
		}
		principal, err := verifier.VerifyAccessToken(ctx.Request.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			adminapi.Error(ctx, err)
			ctx.Abort()
			return
		}
		ctx.Request = ctx.Request.WithContext(auth.WithPrincipal(ctx.Request.Context(), principal))
		ctx.Next()
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal, ok := auth.PrincipalFrom(ctx.Request.Context())
		if !ok || !principal.Has(permission) {
			adminapi.Error(ctx, apperror.Forbidden(apperror.CodePermissionDenied, "无操作权限"))
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
