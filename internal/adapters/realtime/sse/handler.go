package sse

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type Handler struct {
	hub *Hub
	bus eventBus
}

type eventBus interface {
	UserConnected(context.Context, int64) error
	UserDisconnected(context.Context, int64) error
	Heartbeat(context.Context, int64) error
	OnlineCount(context.Context) (int, error)
}

func NewHandler(hub *Hub, bus eventBus) *Handler { return &Handler{hub: hub, bus: bus} }

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	sse := router.Group("/sse")
	sse.GET("/connect", h.connect)
	sse.GET("/online-count", h.onlineCount)
}

func (h *Handler) connect(ctx *gin.Context) {
	userID, ok := adminapi.AccountID(ctx)
	if !ok {
		adminapi.Error(ctx, apperror.Unauthorized(apperror.CodeInvalidAccessToken, "访问令牌无效或已过期"))
		return
	}
	ctx.Header("Content-Type", "text/event-stream; charset=utf-8")
	ctx.Header("Cache-Control", "no-cache, no-transform")
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Header("Content-Encoding", "identity")
	controller := http.NewResponseController(ctx.Writer)
	_ = controller.SetWriteDeadline(time.Time{})
	client, err := h.hub.Connect(userID)
	if err != nil {
		adminapi.Error(ctx, apperror.Internal(err))
		return
	}
	if err := h.bus.UserConnected(ctx.Request.Context(), userID); err != nil {
		h.hub.Disconnect(client)
		adminapi.Error(ctx, apperror.Internal(err))
		return
	}
	defer func() {
		h.hub.Disconnect(client)
		if !h.hub.UserOnline(userID) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = h.bus.UserDisconnected(cleanupCtx, userID)
		}
	}()
	ctx.Status(http.StatusOK)
	if err := controller.Flush(); err != nil {
		return
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-client.Done():
			return
		case message := <-client.messages():
			if client.isClosed() || ctx.Request.Context().Err() != nil {
				return
			}
			_ = controller.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err := ctx.Writer.Write(message); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		case <-ticker.C:
			_ = h.bus.Heartbeat(ctx.Request.Context(), userID)
			if err := client.heartbeat(); err != nil {
				return
			}
		}
	}
}

func (h *Handler) onlineCount(ctx *gin.Context) {
	count, err := h.bus.OnlineCount(ctx.Request.Context())
	if err != nil {
		adminapi.Error(ctx, apperror.Internal(err))
		return
	}
	adminapi.OK(ctx, count)
}
