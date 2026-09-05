package configuration

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	configurationapp "github.com/yuhang1130/go-service-main/internal/features/configuration/application"
	configurationdomain "github.com/yuhang1130/go-service-main/internal/features/configuration/domain"
)

type Handler struct{ service *configurationapp.Service }

func NewHandler(service *configurationapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	configs := router.Group("/configs")
	configs.PUT("/refresh", middleware.RequirePermission("sys:config:update"), h.refresh)
	configs.POST("/refresh", middleware.RequirePermission("sys:config:update"), h.refresh)
	configs.POST("/refresh/:key", middleware.RequirePermission("sys:config:update"), h.refreshKey)
	configs.GET("/key/:key", middleware.RequirePermission("sys:config:list"), h.byKey)
	configs.GET("", middleware.RequirePermission("sys:config:list"), h.list)
	configs.POST("", middleware.RequirePermission("sys:config:create"), h.create)
	configs.GET("/:id/form", middleware.RequirePermission("sys:config:update"), h.form)
	configs.GET("/:id", middleware.RequirePermission("sys:config:list"), h.detail)
	configs.PUT("/:id", middleware.RequirePermission("sys:config:update"), h.update)
	configs.DELETE("/:ids", middleware.RequirePermission("sys:config:delete"), h.delete)
}

func (h *Handler) list(ctx *gin.Context) {
	items, total, err := h.service.List(ctx.Request.Context(), configurationapp.Query{Page: queryInt(ctx, "pageNum", 1), PageSize: queryInt(ctx, "pageSize", 10), Keywords: ctx.Query("keywords")})
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	result := make([]gin.H, len(items))
	for index, item := range items {
		result[index] = response(item)
	}
	adminapi.Page(ctx, result, total)
}

func (h *Handler) form(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	item, err := h.service.Get(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, response(item))
}

func (h *Handler) detail(ctx *gin.Context) { h.form(ctx) }

func (h *Handler) byKey(ctx *gin.Context) {
	item, err := h.service.GetByKey(ctx.Request.Context(), ctx.Param("key"))
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, response(item))
}

func (h *Handler) create(ctx *gin.Context) { h.save(ctx, 0, "保存成功") }

func (h *Handler) update(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if ok {
		h.save(ctx, id, "修改成功")
	}
}

func (h *Handler) save(ctx *gin.Context, id int64, message string) {
	var request configRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "系统配置参数无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.Save(ctx.Request.Context(), configurationapp.Command{ID: id, Name: request.ConfigName, Key: request.ConfigKey, Value: request.ConfigValue, Remark: request.Remark}, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) delete(ctx *gin.Context) {
	ids, err := adminapi.ParseIDs(ctx.Param("ids"))
	if err != nil || len(ids) == 0 {
		adminapi.Invalid(ctx, "系统配置ID无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if len(ids) == 1 {
		err = h.service.Delete(ctx.Request.Context(), ids[0], actorID)
	} else {
		err = h.service.DeleteMany(ctx.Request.Context(), ids, actorID)
	}
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "删除成功")
}

func (h *Handler) refreshKey(ctx *gin.Context) {
	if err := h.service.RefreshKey(ctx.Request.Context(), ctx.Param("key")); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "刷新成功")
}

func (h *Handler) refresh(ctx *gin.Context) {
	if err := h.service.Refresh(ctx.Request.Context()); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "刷新成功")
}

type configRequest struct {
	ConfigName  string `json:"configName"`
	ConfigKey   string `json:"configKey"`
	ConfigValue string `json:"configValue"`
	Remark      string `json:"remark"`
}

func response(item configurationdomain.Config) gin.H {
	return gin.H{"id": strconv.FormatInt(item.ID, 10), "configName": item.Name, "configKey": item.Key, "configValue": item.Value, "remark": item.Remark, "createTime": formatTime(item.CreateTime), "updateTime": formatTime(item.UpdateTime)}
}

func pathID(ctx *gin.Context) (int64, bool) {
	id, err := adminapi.ParseID(ctx.Param("id"))
	if err != nil || id <= 0 {
		adminapi.Invalid(ctx, "系统配置ID无效")
		return 0, false
	}
	return id, true
}

func queryInt(ctx *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(ctx.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}
