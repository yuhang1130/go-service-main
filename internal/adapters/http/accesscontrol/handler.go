package accesscontrol

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	accessapp "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/application"
	accessdomain "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type Handler struct{ service *accessapp.Service }

func NewHandler(service *accessapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	roles := router.Group("/roles")
	roles.GET("", middleware.RequirePermission("sys:role:list"), h.listRoles)
	roles.GET("/options", h.roleOptions)
	roles.POST("", middleware.RequirePermission("sys:role:create"), h.createRole)
	roles.GET("/:id/form", middleware.RequirePermission("sys:role:update"), h.roleForm)
	roles.PUT("/:id", middleware.RequirePermission("sys:role:update"), h.updateRole)
	roles.DELETE("/:ids", middleware.RequirePermission("sys:role:delete"), h.deleteRole)
	roles.GET("/:id/menu-ids", middleware.RequirePermission("sys:role:update"), h.roleMenuIDs)
	roles.PUT("/:id/menus", middleware.RequirePermission("sys:role:assign"), h.setRoleMenus)
	roles.GET("/:id/dept-ids", middleware.RequirePermission("sys:role:update"), h.roleDepartmentIDs)
	roles.PUT("/:id/depts", middleware.RequirePermission("sys:role:update"), h.setRoleDepartments)

	menus := router.Group("/menus")
	menus.GET("/routes", h.routes)
	menus.GET("/options", h.menuOptions)
	menus.GET("", middleware.RequirePermission("sys:menu:list"), h.listMenus)
	menus.POST("", middleware.RequirePermission("sys:menu:create"), h.createMenu)
	menus.GET("/:id/form", middleware.RequirePermission("sys:menu:update"), h.menuForm)
	menus.PUT("/:id", middleware.RequirePermission("sys:menu:update"), h.updateMenu)
	menus.DELETE("/:id", middleware.RequirePermission("sys:menu:delete"), h.deleteMenu)
}

func (h *Handler) listRoles(ctx *gin.Context) {
	query := accessapp.PageQuery{Page: positiveQuery(ctx, "pageNum", 1), PageSize: positiveQuery(ctx, "pageSize", 10), Keywords: ctx.Query("keywords")}
	items, total, err := h.service.ListRoles(ctx.Request.Context(), query)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	responses := make([]roleResponse, len(items))
	for index, item := range items {
		responses[index] = roleToResponse(item)
	}
	adminapi.Page(ctx, responses, total)
}

func (h *Handler) roleOptions(ctx *gin.Context) {
	items, err := h.service.RoleOptions(ctx.Request.Context())
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	options := make([]gin.H, len(items))
	for index, item := range items {
		options[index] = gin.H{"value": strconv.FormatInt(item.ID, 10), "label": item.Name}
	}
	adminapi.OK(ctx, options)
}

func (h *Handler) roleForm(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	item, err := h.service.GetRole(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, roleToForm(item))
}

func (h *Handler) createRole(ctx *gin.Context) { h.saveRole(ctx, 0, "保存成功") }

func (h *Handler) updateRole(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if ok {
		h.saveRole(ctx, id, "修改成功")
	}
}

func (h *Handler) saveRole(ctx *gin.Context, id int64, message string) {
	var request roleRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "角色参数无效")
		return
	}
	command := accessapp.RoleCommand{ID: id, Name: request.Name, Code: request.Code, Sort: request.Sort, Status: request.Status, DataScope: request.DataScope, DepartmentIDs: idsToInt64(request.DepartmentIDs), MenuIDs: idsToInt64(request.MenuIDs)}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.SaveRole(ctx.Request.Context(), command, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) deleteRole(ctx *gin.Context) {
	ids, err := adminapi.ParseIDs(ctx.Param("ids"))
	if err != nil {
		adminapi.Invalid(ctx, "角色ID无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	for _, id := range ids {
		if err := h.service.DeleteRole(ctx.Request.Context(), id, actorID); err != nil {
			adminapi.Error(ctx, err)
			return
		}
	}
	adminapi.OKMessage(ctx, "删除成功")
}

func (h *Handler) roleMenuIDs(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	ids, err := h.service.RoleMenuIDs(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, stringIDs(ids))
}

func (h *Handler) setRoleMenus(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	var ids []int64
	if err := ctx.ShouldBindJSON(&ids); err != nil {
		adminapi.Invalid(ctx, "菜单ID列表无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.SetRoleMenus(ctx.Request.Context(), id, ids, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "分配成功")
}

func (h *Handler) roleDepartmentIDs(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	ids, err := h.service.RoleDepartmentIDs(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, stringIDs(ids))
}

func (h *Handler) setRoleDepartments(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	var ids []int64
	if err := ctx.ShouldBindJSON(&ids); err != nil {
		adminapi.Invalid(ctx, "部门ID列表无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.SetRoleDepartments(ctx.Request.Context(), id, ids, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "分配成功")
}

func (h *Handler) listMenus(ctx *gin.Context) {
	items, err := h.service.ListMenus(ctx.Request.Context(), accessapp.MenuQuery{Keywords: ctx.Query("keywords")})
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, menusToResponse(items))
}

func (h *Handler) menuOptions(ctx *gin.Context) {
	items, err := h.service.MenuOptions(ctx.Request.Context(), ctx.Query("onlyParent") == "true")
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, menusToOptions(items))
}

func (h *Handler) routes(ctx *gin.Context) {
	accountID, ok := adminapi.AccountID(ctx)
	if !ok {
		adminapi.Error(ctx, apperror.Unauthorized("A0230", "访问令牌无效或已过期"))
		return
	}
	routes, err := h.service.Routes(ctx.Request.Context(), accountID)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, routes)
}

func (h *Handler) menuForm(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	item, err := h.service.GetMenu(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, menuToResponse(&item))
}

func (h *Handler) createMenu(ctx *gin.Context) { h.saveMenu(ctx, 0, "保存成功") }

func (h *Handler) updateMenu(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if ok {
		h.saveMenu(ctx, id, "修改成功")
	}
}

func (h *Handler) saveMenu(ctx *gin.Context, id int64, message string) {
	var request menuRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "菜单参数无效")
		return
	}
	params := make(map[string]any, len(request.Params))
	for _, pair := range request.Params {
		if pair.Key != "" {
			params[pair.Key] = pair.Value
		}
	}
	command := accessapp.MenuCommand{ID: id, ParentID: int64(request.ParentID), Name: request.Name, Type: request.Type, RouteName: request.RouteName, RoutePath: request.RoutePath, Component: request.Component, ExternalURL: request.ExternalURL, Permission: request.Permission, AlwaysShow: int(request.AlwaysShow), KeepAlive: int(request.KeepAlive), Visible: request.Visible, Sort: request.Sort, Icon: request.Icon, Redirect: request.Redirect, Params: params}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.SaveMenu(ctx.Request.Context(), command, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) deleteMenu(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.DeleteMenu(ctx.Request.Context(), id, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "删除成功")
}

type roleRequest struct {
	Name          string        `json:"name"`
	Code          string        `json:"code"`
	Sort          int           `json:"sort"`
	Status        int           `json:"status"`
	DataScope     int           `json:"dataScope"`
	DepartmentIDs []adminapi.ID `json:"deptIds"`
	MenuIDs       []adminapi.ID `json:"menuIds"`
}

type roleResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Code           string `json:"code"`
	Sort           int    `json:"sort"`
	Status         int    `json:"status"`
	DataScope      int    `json:"dataScope"`
	DataScopeLabel string `json:"dataScopeLabel"`
	UpdateTime     string `json:"updateTime"`
}

func roleToResponse(role accessdomain.Role) roleResponse {
	return roleResponse{ID: strconv.FormatInt(role.ID, 10), Name: role.Name, Code: role.Code, Sort: role.Sort, Status: role.Status, DataScope: role.DataScope, DataScopeLabel: dataScopeLabel(role.DataScope), UpdateTime: formatDateTime(role.UpdateTime)}
}

func roleToForm(role accessdomain.Role) gin.H {
	deptIDs := make([]string, len(role.DepartmentIDs))
	for index, id := range role.DepartmentIDs {
		deptIDs[index] = strconv.FormatInt(id, 10)
	}
	return gin.H{"id": strconv.FormatInt(role.ID, 10), "name": role.Name, "code": role.Code, "sort": role.Sort, "status": role.Status, "dataScope": role.DataScope, "deptIds": deptIDs}
}

type keyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type menuRequest struct {
	ParentID    adminapi.ID   `json:"parentId"`
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	RouteName   string        `json:"routeName"`
	RoutePath   string        `json:"routePath"`
	Component   string        `json:"component"`
	ExternalURL string        `json:"externalUrl"`
	Permission  string        `json:"perm"`
	AlwaysShow  adminapi.Flag `json:"alwaysShow"`
	KeepAlive   adminapi.Flag `json:"keepAlive"`
	Visible     int           `json:"visible"`
	Sort        int           `json:"sort"`
	Icon        string        `json:"icon"`
	Redirect    string        `json:"redirect"`
	Params      []keyValue    `json:"params"`
}

func menusToResponse(items []*accessdomain.Menu) []gin.H {
	result := make([]gin.H, len(items))
	for index, item := range items {
		result[index] = menuToResponse(item)
	}
	return result
}

func menuToResponse(item *accessdomain.Menu) gin.H {
	pairs := make([]keyValue, 0, len(item.Params))
	for key, value := range item.Params {
		pairs = append(pairs, keyValue{Key: key, Value: toString(value)})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
	response := gin.H{"id": strconv.FormatInt(item.ID, 10), "parentId": strconv.FormatInt(item.ParentID, 10), "name": item.Name, "type": item.Type, "routeName": item.RouteName, "routePath": item.RoutePath, "component": item.Component, "externalUrl": item.ExternalURL, "perm": item.Permission, "alwaysShow": item.AlwaysShow, "keepAlive": item.KeepAlive, "visible": item.Visible, "sort": item.Sort, "icon": item.Icon, "redirect": item.Redirect, "params": pairs, "createTime": formatDateTime(item.CreateTime), "updateTime": formatDateTime(item.UpdateTime)}
	if len(item.Children) > 0 {
		response["children"] = menusToResponse(item.Children)
	}
	return response
}

func menusToOptions(items []*accessdomain.Menu) []gin.H {
	result := make([]gin.H, len(items))
	for index, item := range items {
		option := gin.H{"value": strconv.FormatInt(item.ID, 10), "label": item.Name}
		if len(item.Children) > 0 {
			option["children"] = menusToOptions(item.Children)
		}
		result[index] = option
	}
	return result
}

func pathID(ctx *gin.Context) (int64, bool) {
	id, err := adminapi.ParseID(ctx.Param("id"))
	if err != nil || id <= 0 {
		adminapi.Invalid(ctx, "ID无效")
		return 0, false
	}
	return id, true
}

func positiveQuery(ctx *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(ctx.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func idsToInt64(ids []adminapi.ID) []int64 {
	result := make([]int64, len(ids))
	for index, id := range ids {
		result[index] = int64(id)
	}
	return result
}

func stringIDs(ids []int64) []string {
	result := make([]string, len(ids))
	for index, id := range ids {
		result[index] = strconv.FormatInt(id, 10)
	}
	return result
}

func dataScopeLabel(scope int) string {
	return map[int]string{1: "全部数据", 2: "部门及子部门", 3: "本部门", 4: "本人", 5: "自定义部门"}[scope]
}

func formatDateTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func toString(value any) string { return fmt.Sprint(value) }
