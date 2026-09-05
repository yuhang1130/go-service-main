package organization

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	organizationapp "github.com/yuhang1130/go-service-main/internal/features/organization/application"
	organizationdomain "github.com/yuhang1130/go-service-main/internal/features/organization/domain"
)

type Handler struct{ service *organizationapp.Service }

func NewHandler(service *organizationapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	departments := router.Group("/depts")
	departments.GET("/options", h.options)
	departments.GET("", middleware.RequirePermission("sys:dept:list"), h.list)
	departments.POST("", middleware.RequirePermission("sys:dept:create"), h.create)
	departments.GET("/:id/form", middleware.RequirePermission("sys:dept:update"), h.form)
	departments.PUT("/:id", middleware.RequirePermission("sys:dept:update"), h.update)
	departments.DELETE("/:ids", middleware.RequirePermission("sys:dept:delete"), h.delete)
}

func (h *Handler) list(ctx *gin.Context) {
	query := organizationapp.Query{Keywords: ctx.Query("keywords")}
	if raw := ctx.Query("status"); raw != "" {
		status, err := strconv.Atoi(raw)
		if err != nil {
			adminapi.Invalid(ctx, "部门状态无效")
			return
		}
		query.Status = &status
	}
	items, err := h.service.List(ctx.Request.Context(), query)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, departmentsToResponse(items))
}

func (h *Handler) options(ctx *gin.Context) {
	items, err := h.service.Options(ctx.Request.Context())
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, departmentsToOptions(items))
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
	adminapi.OK(ctx, departmentToForm(item))
}

func (h *Handler) create(ctx *gin.Context) { h.save(ctx, 0, "保存成功") }

func (h *Handler) update(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if ok {
		h.save(ctx, id, "修改成功")
	}
}

func (h *Handler) save(ctx *gin.Context, id int64, message string) {
	var request departmentRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "部门参数无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	command := organizationapp.SaveCommand{ID: id, Name: request.Name, Code: request.Code, ParentID: int64(request.ParentID), Sort: request.Sort, Status: request.Status}
	var err error
	if id == 0 {
		err = h.service.Create(ctx.Request.Context(), command, actorID)
	} else {
		err = h.service.Update(ctx.Request.Context(), command, actorID)
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
		adminapi.Invalid(ctx, "部门ID无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	for _, id := range ids {
		if err := h.service.Delete(ctx.Request.Context(), id, actorID); err != nil {
			adminapi.Error(ctx, err)
			return
		}
	}
	adminapi.OKMessage(ctx, "删除成功")
}

type departmentRequest struct {
	Name     string      `json:"name"`
	Code     string      `json:"code"`
	ParentID adminapi.ID `json:"parentId"`
	Sort     int         `json:"sort"`
	Status   int         `json:"status"`
}

func departmentsToResponse(items []*organizationdomain.Department) []gin.H {
	result := make([]gin.H, len(items))
	for index, item := range items {
		response := gin.H{"id": strconv.FormatInt(item.ID, 10), "name": item.Name, "parentId": strconv.FormatInt(item.ParentID, 10), "treePath": item.TreePath, "sort": item.Sort, "status": item.Status, "createTime": formatTime(item.CreateTime), "updateTime": formatTime(item.UpdateTime)}
		if len(item.Children) > 0 {
			response["children"] = departmentsToResponse(item.Children)
		}
		result[index] = response
	}
	return result
}

func departmentsToOptions(items []*organizationdomain.Department) []gin.H {
	result := make([]gin.H, len(items))
	for index, item := range items {
		option := gin.H{"value": strconv.FormatInt(item.ID, 10), "label": item.Name}
		if len(item.Children) > 0 {
			option["children"] = departmentsToOptions(item.Children)
		}
		result[index] = option
	}
	return result
}

func departmentToForm(item organizationdomain.Department) gin.H {
	return gin.H{"id": strconv.FormatInt(item.ID, 10), "name": item.Name, "code": item.Code, "parentId": strconv.FormatInt(item.ParentID, 10), "sort": item.Sort, "status": item.Status}
}

func pathID(ctx *gin.Context) (int64, bool) {
	id, err := adminapi.ParseID(ctx.Param("id"))
	if err != nil || id <= 0 {
		adminapi.Invalid(ctx, "部门ID无效")
		return 0, false
	}
	return id, true
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}
