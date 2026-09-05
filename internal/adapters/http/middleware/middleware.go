package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader("X-Request-ID")
		if _, err := uuid.Parse(requestID); err != nil {
			requestID = uuid.NewString()
		}
		ctx.Set(requestIDKey, requestID)
		ctx.Header("X-Request-ID", requestID)
		ctx.Next()
	}
}

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "request_id", RequestIDValue(ctx), "panic", recovered, "stack", string(debug.Stack()))
				WriteError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
				ctx.Abort()
			}
		}()
		ctx.Next()
	}
}

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		started := time.Now()
		ctx.Next()
		logger.Info("http request",
			"request_id", RequestIDValue(ctx),
			"method", ctx.Request.Method,
			"path", ctx.FullPath(),
			"status", ctx.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
		)
	}
}

func BodySize(limit int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if limit > 0 {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limit)
		}
		ctx.Next()
	}
}

func RequestIDValue(ctx *gin.Context) string {
	value, _ := ctx.Get(requestIDKey)
	requestID, _ := value.(string)
	return requestID
}

func WriteError(ctx *gin.Context, status int, code, message string) {
	ctx.JSON(status, gin.H{"code": code, "message": message, "request_id": RequestIDValue(ctx)})
}
