package dictionary

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	dictionaryapp "github.com/yuhang1130/go-service-main/internal/features/dictionary/application"
	dictionarydomain "github.com/yuhang1130/go-service-main/internal/features/dictionary/domain"
)

type Handler struct{ service *dictionaryapp.Service }

func NewHandler(service *dictionaryapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	dictionaries := router.Group("/dicts")
	dictionaries.GET("/options", h.options)
	dictionaries.GET("", middleware.RequirePermission("sys:dict:list"), h.list)
	dictionaries.POST("", middleware.RequirePermission("sys:dict:create"), h.create)
	dictionaries.GET("/:dictCode/items/options", h.itemOptions)
	dictionaries.GET("/:dictCode/items/:itemId/form", middleware.RequirePermission("sys:dict-item:update"), h.itemForm)
	dictionaries.PUT("/:dictCode/items/:itemId", middleware.RequirePermission("sys:dict-item:update"), h.updateItem)
	dictionaries.DELETE("/:dictCode/items/:itemIds", middleware.RequirePermission("sys:dict-item:delete"), h.deleteItems)
	dictionaries.GET("/:dictCode/items", middleware.RequirePermission("sys:dict-item:list"), h.listItems)
	dictionaries.POST("/:dictCode/items", middleware.RequirePermission("sys:dict-item:create"), h.createItem)
	dictionaries.GET("/:dictCode/form", middleware.RequirePermission("sys:dict:update"), h.form)
	dictionaries.PUT("/:dictCode", middleware.RequirePermission("sys:dict:update"), h.update)
	dictionaries.DELETE("/:dictCode", middleware.RequirePermission("sys:dict:delete"), h.delete)
}

func (h *Handler) list(ctx *gin.Context) {
	query := dictionaryapp.Query{Page: queryInt(ctx, "pageNum", 1), PageSize: queryInt(ctx, "pageSize", 10), Keywords: ctx.Query("keywords")}
	if value, ok := optionalInt(ctx.Query("status")); ok {
		query.Status = &value
	}
	items, total, err := h.service.List(ctx.Request.Context(), query)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	result := make([]gin.H, len(items))
	for index, item := range items {
		result[index] = dictionaryResponse(item)
	}
	adminapi.Page(ctx, result, total)
}

func (h *Handler) options(ctx *gin.Context) {
	items, err := h.service.Options(ctx.Request.Context())
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	result := make([]gin.H, len(items))
	for index, item := range items {
		result[index] = gin.H{"value": item.Code, "label": item.Name}
	}
	adminapi.OK(ctx, result)
}

func (h *Handler) form(ctx *gin.Context) {
	id, ok := pathID(ctx, "dictCode", "字典ID无效")
	if !ok {
		return
	}
	item, err := h.service.Get(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, dictionaryResponse(item))
}

func (h *Handler) create(ctx *gin.Context) { h.save(ctx, 0, "保存成功") }

func (h *Handler) update(ctx *gin.Context) {
	id, ok := pathID(ctx, "dictCode", "字典ID无效")
	if ok {
		h.save(ctx, id, "修改成功")
	}
}

func (h *Handler) save(ctx *gin.Context, id int64, message string) {
	var request dictionaryRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "字典参数无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.Save(ctx.Request.Context(), dictionaryapp.DictionaryCommand{ID: id, Code: request.DictCode, Name: request.Name, Status: request.Status, Remark: request.Remark}, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) delete(ctx *gin.Context) {
	id, ok := pathID(ctx, "dictCode", "字典ID无效")
	if !ok {
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.Delete(ctx.Request.Context(), id, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "删除成功")
}

func (h *Handler) listItems(ctx *gin.Context) {
	query := dictionaryapp.ItemQuery{Page: queryInt(ctx, "pageNum", 1), PageSize: queryInt(ctx, "pageSize", 10), DictCode: ctx.Param("dictCode"), Keywords: ctx.Query("keywords")}
	if value, ok := optionalInt(ctx.Query("status")); ok {
		query.Status = &value
	}
	items, total, err := h.service.ListItems(ctx.Request.Context(), query)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.Page(ctx, itemsResponse(items), total)
}

func (h *Handler) itemOptions(ctx *gin.Context) {
	items, err := h.service.ItemOptions(ctx.Request.Context(), ctx.Param("dictCode"))
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, itemsResponse(items))
}

func (h *Handler) itemForm(ctx *gin.Context) {
	id, ok := pathID(ctx, "itemId", "字典项ID无效")
	if !ok {
		return
	}
	item, err := h.service.GetItem(ctx.Request.Context(), id, ctx.Param("dictCode"))
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, itemResponse(item))
}

func (h *Handler) createItem(ctx *gin.Context) { h.saveItem(ctx, 0, "新增成功") }

func (h *Handler) updateItem(ctx *gin.Context) {
	id, ok := pathID(ctx, "itemId", "字典项ID无效")
	if ok {
		h.saveItem(ctx, id, "更新成功")
	}
}

func (h *Handler) saveItem(ctx *gin.Context, id int64, message string) {
	var request itemRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "字典项参数无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	command := dictionaryapp.ItemCommand{ID: id, DictCode: ctx.Param("dictCode"), Value: request.Value, Label: request.Label, TagType: request.TagType, Sort: request.Sort, Status: request.Status, Remark: request.Remark}
	if err := h.service.SaveItem(ctx.Request.Context(), command, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) deleteItems(ctx *gin.Context) {
	ids, err := adminapi.ParseIDs(ctx.Param("itemIds"))
	if err != nil {
		adminapi.Invalid(ctx, "字典项ID无效")
		return
	}
	if err := h.service.DeleteItems(ctx.Request.Context(), ctx.Param("dictCode"), ids); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "删除成功")
}

type dictionaryRequest struct {
	DictCode string `json:"dictCode"`
	Name     string `json:"name"`
	Status   int    `json:"status"`
	Remark   string `json:"remark"`
}

type itemRequest struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	TagType string `json:"tagType"`
	Sort    int    `json:"sort"`
	Status  int    `json:"status"`
	Remark  string `json:"remark"`
}

func dictionaryResponse(item dictionarydomain.Dictionary) gin.H {
	return gin.H{"id": strconv.FormatInt(item.ID, 10), "dictCode": item.Code, "name": item.Name, "status": item.Status, "remark": item.Remark, "createTime": formatTime(item.CreateTime), "updateTime": formatTime(item.UpdateTime)}
}

func itemsResponse(items []dictionarydomain.Item) []gin.H {
	result := make([]gin.H, len(items))
	for index, item := range items {
		result[index] = itemResponse(item)
	}
	return result
}

func itemResponse(item dictionarydomain.Item) gin.H {
	return gin.H{"id": strconv.FormatInt(item.ID, 10), "dictCode": item.DictCode, "value": item.Value, "label": item.Label, "tagType": item.TagType, "sort": item.Sort, "status": item.Status, "remark": item.Remark}
}

func queryInt(ctx *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(ctx.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func optionalInt(raw string) (int, bool) {
	value, err := strconv.Atoi(raw)
	return value, raw != "" && err == nil
}

func pathID(ctx *gin.Context, key, message string) (int64, bool) {
	id, err := adminapi.ParseID(ctx.Param(key))
	if err != nil || id <= 0 {
		adminapi.Invalid(ctx, message)
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
