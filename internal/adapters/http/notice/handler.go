package notice

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
)

type Handler struct{ service *noticeapp.Service }

func NewHandler(service *noticeapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	notices := router.Group("/notices")
	notices.GET("/my", h.listMine)
	notices.PUT("/read-all", h.readAll)
	notices.GET("/unread-count", h.unreadCount)
	notices.GET("", middleware.RequirePermission("sys:notice:list"), h.list)
	notices.POST("", middleware.RequirePermission("sys:notice:create"), h.create)
	notices.GET("/:id/form", middleware.RequirePermission("sys:notice:update"), h.form)
	notices.GET("/:id/detail", h.detail)
	notices.PUT("/:id/publish", middleware.RequirePermission("sys:notice:publish"), h.publish)
	notices.PUT("/:id/revoke", middleware.RequirePermission("sys:notice:publish"), h.revoke)
	notices.PUT("/:id", middleware.RequirePermission("sys:notice:update"), h.update)
	notices.DELETE("/:ids", middleware.RequirePermission("sys:notice:delete"), h.delete)
}

func (h *Handler) list(ctx *gin.Context) {
	query, ok := parseQuery(ctx)
	if !ok {
		return
	}
	items, total, err := h.service.List(ctx.Request.Context(), query)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.Page(ctx, responses(items), total)
}

func (h *Handler) listMine(ctx *gin.Context) {
	userID, ok := adminapi.AccountID(ctx)
	if !ok {
		adminapi.Invalid(ctx, "当前用户无效")
		return
	}
	query, valid := parseQuery(ctx)
	if !valid {
		return
	}
	items, total, err := h.service.ListMine(ctx.Request.Context(), userID, query)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.Page(ctx, responses(items), total)
}

func (h *Handler) form(ctx *gin.Context) {
	h.get(ctx, true, false)
}

func (h *Handler) detail(ctx *gin.Context) {
	h.get(ctx, false, true)
}

func (h *Handler) get(ctx *gin.Context, manager, detail bool) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	userID, _ := adminapi.AccountID(ctx)
	item, err := h.service.Get(ctx.Request.Context(), id, userID, manager)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	response := noticeResponse(item)
	if !detail {
		response["status"] = item.PublishStatus
		response["targetUsers"] = item.TargetUserIDs
	}
	adminapi.OK(ctx, response)
}

func (h *Handler) create(ctx *gin.Context) { h.save(ctx, 0, "保存成功") }

func (h *Handler) update(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if ok {
		h.save(ctx, id, "修改成功")
	}
}

func (h *Handler) save(ctx *gin.Context, id int64, message string) {
	var request noticeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "通知参数无效")
		return
	}
	targets := request.TargetUserIDs
	if len(targets) == 0 {
		targets = request.TargetUsers
	}
	ids := make([]int64, len(targets))
	for index, target := range targets {
		ids[index] = int64(target)
	}
	actorID, _ := adminapi.AccountID(ctx)
	command := noticeapp.Command{ID: id, Title: request.Title, Content: request.Content, Type: request.Type, Level: request.Level, Status: request.Status, TargetType: request.TargetType, TargetUserIDs: ids}
	if err := h.service.Save(ctx.Request.Context(), command, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) publish(ctx *gin.Context) {
	h.changeState(ctx, true)
}

func (h *Handler) revoke(ctx *gin.Context) {
	h.changeState(ctx, false)
}

func (h *Handler) changeState(ctx *gin.Context, publish bool) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	var err error
	message := "撤回成功"
	if publish {
		err = h.service.Publish(ctx.Request.Context(), id, actorID)
		message = "发布成功"
	} else {
		err = h.service.Revoke(ctx.Request.Context(), id, actorID)
	}
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) delete(ctx *gin.Context) {
	ids, err := adminapi.ParseIDs(ctx.Param("ids"))
	if err != nil {
		adminapi.Invalid(ctx, "通知ID无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.Delete(ctx.Request.Context(), ids, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "删除成功")
}

func (h *Handler) readAll(ctx *gin.Context) {
	userID, _ := adminapi.AccountID(ctx)
	if err := h.service.ReadAll(ctx.Request.Context(), userID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "全部已读成功")
}

func (h *Handler) unreadCount(ctx *gin.Context) {
	userID, _ := adminapi.AccountID(ctx)
	count, err := h.service.UnreadCount(ctx.Request.Context(), userID)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, gin.H{"count": count})
}

type noticeRequest struct {
	Title         string        `json:"title"`
	Content       string        `json:"content"`
	Type          int           `json:"type"`
	Level         string        `json:"level"`
	Status        int           `json:"status"`
	TargetType    int           `json:"targetType"`
	TargetUsers   []adminapi.ID `json:"targetUsers"`
	TargetUserIDs []adminapi.ID `json:"targetUserIds"`
}

func parseQuery(ctx *gin.Context) (noticeapp.Query, bool) {
	query := noticeapp.Query{Page: queryInt(ctx, "pageNum", 1), PageSize: queryInt(ctx, "pageSize", 10), Title: ctx.Query("title")}
	for _, candidate := range []struct {
		name   string
		target **int
	}{
		{name: "type", target: &query.Type},
		{name: "isRead", target: &query.IsRead},
	} {
		if raw := ctx.Query(candidate.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				adminapi.Invalid(ctx, "通知查询参数无效")
				return noticeapp.Query{}, false
			}
			*candidate.target = &value
		}
	}
	statusRaw := ctx.Query("publishStatus")
	if statusRaw == "" {
		statusRaw = ctx.Query("status")
	}
	if statusRaw != "" {
		value, err := strconv.Atoi(statusRaw)
		if err != nil {
			adminapi.Invalid(ctx, "通知发布状态无效")
			return noticeapp.Query{}, false
		}
		query.Status = &value
	}
	return query, true
}

func responses(items []noticedomain.Notice) []gin.H {
	result := make([]gin.H, len(items))
	for index, item := range items {
		result[index] = noticeResponse(item)
	}
	return result
}

func noticeResponse(item noticedomain.Notice) gin.H {
	return gin.H{"id": strconv.FormatInt(item.ID, 10), "title": item.Title, "content": item.Content, "type": item.Type, "level": item.Level, "publishStatus": item.PublishStatus, "targetType": item.TargetType, "targetUserIds": item.TargetUserIDs, "publisherName": item.PublisherName, "publishTime": formatOptionalTime(item.PublishTime), "revokeTime": formatOptionalTime(item.RevokeTime), "isRead": item.IsRead, "createTime": item.CreateTime.Local().Format("2006-01-02 15:04:05")}
}

func pathID(ctx *gin.Context) (int64, bool) {
	id, err := adminapi.ParseID(ctx.Param("id"))
	if err != nil || id <= 0 {
		adminapi.Invalid(ctx, "通知ID无效")
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

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}
