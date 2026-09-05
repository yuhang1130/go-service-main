package identity

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	identitydomain "github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type Handler struct{ service *identityapp.Service }

func NewHandler(service *identityapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterPublic(router *gin.RouterGroup) {
	router.GET("/auth/captcha", h.captcha)
	router.POST("/auth/login", h.login)
	router.POST("/auth/refresh-token", h.refresh)
}

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	router.DELETE("/auth/logout", h.logout)
	users := router.Group("/users")
	users.GET("/me", h.current)
	users.GET("/profile", h.profile)
	users.PUT("/profile", h.updateProfile)
	users.PUT("/password", h.changePassword)
	users.GET("/options", h.options)
	users.GET("/template", middleware.RequirePermission("sys:user:import"), h.downloadTemplate)
	users.GET("/export", middleware.RequirePermission("sys:user:export"), h.exportUsers)
	users.POST("/import", middleware.RequirePermission("sys:user:import"), h.importUsers)
	users.GET("", middleware.RequirePermission("sys:user:list"), h.list)
	users.POST("", middleware.RequirePermission("sys:user:create"), h.create)
	users.GET("/:userId/form", middleware.RequirePermission("sys:user:update"), h.form)
	users.PUT("/:userId", middleware.RequirePermission("sys:user:update"), h.update)
	users.DELETE("/:ids", middleware.RequirePermission("sys:user:delete"), h.delete)
	users.PATCH("/:userId/status", middleware.RequirePermission("sys:user:update"), h.setStatus)
	users.PUT("/:userId/password/reset", middleware.RequirePermission("sys:user:reset-password"), h.resetPassword)
}

func (h *Handler) captcha(ctx *gin.Context) {
	captcha, err := h.service.Captcha(ctx.Request.Context())
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, gin.H{"captchaId": captcha.ID, "captchaBase64": captcha.Image})
}

func (h *Handler) login(ctx *gin.Context) {
	var request struct {
		Username    string `json:"username" binding:"required"`
		Password    string `json:"password" binding:"required"`
		CaptchaID   string `json:"captchaId" binding:"required"`
		CaptchaCode string `json:"captchaCode" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "登录参数无效")
		return
	}
	tokens, err := h.service.Login(ctx.Request.Context(), identityapp.LoginCommand{Username: request.Username, Password: request.Password, CaptchaID: request.CaptchaID, CaptchaCode: request.CaptchaCode})
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, tokens)
}

func (h *Handler) refresh(ctx *gin.Context) {
	token := ctx.Query("refreshToken")
	if token == "" {
		token = ctx.PostForm("refreshToken")
	}
	tokens, err := h.service.Refresh(ctx.Request.Context(), token)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, tokens)
}

func (h *Handler) logout(ctx *gin.Context) {
	token := strings.TrimSpace(strings.TrimPrefix(ctx.GetHeader("Authorization"), "Bearer "))
	if err := h.service.Logout(ctx.Request.Context(), token); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "退出成功")
}

func (h *Handler) current(ctx *gin.Context) {
	accountID, ok := adminapi.AccountID(ctx)
	if !ok {
		adminapi.Error(ctx, apperror.Unauthorized("A0230", "访问令牌无效或已过期"))
		return
	}
	current, err := h.service.Current(ctx.Request.Context(), accountID)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, gin.H{"userId": strconv.FormatInt(current.UserID, 10), "username": current.Username, "nickname": current.Nickname, "avatar": current.Avatar, "roles": current.Roles, "perms": current.Permissions})
}

func (h *Handler) list(ctx *gin.Context) {
	accountID, ok := adminapi.AccountID(ctx)
	if !ok {
		adminapi.Error(ctx, apperror.Unauthorized("A0230", "访问令牌无效或已过期"))
		return
	}
	query, valid := userListQuery(ctx)
	if !valid {
		return
	}
	items, total, err := h.service.List(ctx.Request.Context(), query, accountID)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	responses := make([]accountResponse, len(items))
	for index, item := range items {
		responses[index] = accountToResponse(item)
	}
	adminapi.Page(ctx, responses, total)
}

func userListQuery(ctx *gin.Context) (identityapp.ListQuery, bool) {
	query := identityapp.ListQuery{Page: queryInt(ctx, "pageNum", 1), PageSize: queryInt(ctx, "pageSize", 10), Keywords: ctx.Query("keywords")}
	if value, exists := queryOptionalInt(ctx, "status"); exists {
		query.Status = &value
	}
	if value := ctx.Query("deptId"); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			adminapi.Invalid(ctx, "部门ID无效")
			return identityapp.ListQuery{}, false
		}
		query.DepartmentID = &id
	}
	created := ctx.QueryArray("createTime")
	if len(created) == 2 {
		from, fromErr := time.ParseInLocation("2006-01-02 15:04:05", created[0], time.Local)
		to, toErr := time.ParseInLocation("2006-01-02 15:04:05", created[1], time.Local)
		if fromErr != nil || toErr != nil {
			adminapi.Invalid(ctx, "创建时间范围无效")
			return identityapp.ListQuery{}, false
		}
		query.CreatedFrom, query.CreatedTo = &from, &to
	}
	return query, true
}

func (h *Handler) options(ctx *gin.Context) {
	accountID, ok := adminapi.AccountID(ctx)
	if !ok {
		adminapi.Error(ctx, apperror.Unauthorized("A0230", "访问令牌无效或已过期"))
		return
	}
	items, err := h.service.Options(ctx.Request.Context(), accountID)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	options := make([]gin.H, len(items))
	for index, item := range items {
		options[index] = gin.H{"value": strconv.FormatInt(item.ID, 10), "label": item.Nickname}
	}
	adminapi.OK(ctx, options)
}

func (h *Handler) form(ctx *gin.Context) {
	id, err := adminapi.ParseID(ctx.Param("userId"))
	if err != nil {
		adminapi.Invalid(ctx, "用户ID无效")
		return
	}
	account, err := h.service.Get(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, accountToForm(account))
}

func (h *Handler) create(ctx *gin.Context) { h.save(ctx, 0, "保存成功") }

func (h *Handler) update(ctx *gin.Context) {
	id, err := adminapi.ParseID(ctx.Param("userId"))
	if err != nil {
		adminapi.Invalid(ctx, "用户ID无效")
		return
	}
	h.save(ctx, id, "修改成功")
}

func (h *Handler) save(ctx *gin.Context, id int64, message string) {
	var request accountRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "用户参数无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	roleIDs := make([]int64, len(request.RoleIDs))
	for index, roleID := range request.RoleIDs {
		roleIDs[index] = int64(roleID)
	}
	command := identityapp.SaveCommand{ID: id, Username: request.Username, Nickname: request.Nickname, Mobile: request.Mobile, Gender: request.Gender, Avatar: request.Avatar, Email: request.Email, Status: request.Status, DepartmentID: int64(request.DepartmentID), RoleIDs: roleIDs}
	if err := h.service.Save(ctx.Request.Context(), command, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, message)
}

func (h *Handler) delete(ctx *gin.Context) {
	ids, err := adminapi.ParseIDs(ctx.Param("ids"))
	if err != nil {
		adminapi.Invalid(ctx, "用户ID无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.Delete(ctx.Request.Context(), ids, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "删除成功")
}

func (h *Handler) setStatus(ctx *gin.Context) {
	id, err := adminapi.ParseID(ctx.Param("userId"))
	status, statusErr := strconv.Atoi(ctx.Query("status"))
	if err != nil || statusErr != nil {
		adminapi.Invalid(ctx, "用户ID或状态无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.SetStatus(ctx.Request.Context(), id, status, actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "修改成功")
}

func (h *Handler) resetPassword(ctx *gin.Context) {
	id, err := adminapi.ParseID(ctx.Param("userId"))
	if err != nil {
		adminapi.Invalid(ctx, "用户ID无效")
		return
	}
	actorID, _ := adminapi.AccountID(ctx)
	if err := h.service.ResetPassword(ctx.Request.Context(), id, ctx.Query("password"), actorID); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "密码重置成功")
}

func (h *Handler) profile(ctx *gin.Context) {
	id, _ := adminapi.AccountID(ctx)
	account, err := h.service.Profile(ctx.Request.Context(), id)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, accountToResponse(account))
}

func (h *Handler) updateProfile(ctx *gin.Context) {
	var request struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
		Gender   int    `json:"gender"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "个人资料参数无效")
		return
	}
	id, _ := adminapi.AccountID(ctx)
	if err := h.service.UpdateProfile(ctx.Request.Context(), id, identityapp.ProfileCommand{Nickname: request.Nickname, Avatar: request.Avatar, Gender: request.Gender}); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "修改成功")
}

func (h *Handler) changePassword(ctx *gin.Context) {
	var request struct {
		OldPassword     string `json:"oldPassword"`
		NewPassword     string `json:"newPassword"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		adminapi.Invalid(ctx, "密码参数无效")
		return
	}
	id, _ := adminapi.AccountID(ctx)
	if err := h.service.ChangePassword(ctx.Request.Context(), id, identityapp.PasswordCommand{OldPassword: request.OldPassword, NewPassword: request.NewPassword, ConfirmPassword: request.ConfirmPassword}); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "密码修改成功")
}

type accountRequest struct {
	Username     string        `json:"username"`
	Nickname     string        `json:"nickname"`
	Mobile       string        `json:"mobile"`
	Gender       int           `json:"gender"`
	Avatar       string        `json:"avatar"`
	Email        string        `json:"email"`
	Status       int           `json:"status"`
	DepartmentID adminapi.ID   `json:"deptId"`
	RoleIDs      []adminapi.ID `json:"roleIds"`
}

type accountResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Nickname       string `json:"nickname"`
	Mobile         string `json:"mobile"`
	Gender         int    `json:"gender"`
	Avatar         string `json:"avatar"`
	Email          string `json:"email"`
	Status         int    `json:"status"`
	DepartmentName string `json:"deptName"`
	RoleNames      string `json:"roleNames"`
	CreateTime     string `json:"createTime"`
}

func accountToResponse(account identitydomain.Account) accountResponse {
	return accountResponse{ID: strconv.FormatInt(account.ID, 10), Username: account.Username, Nickname: account.Nickname, Mobile: account.Mobile, Gender: account.Gender, Avatar: account.Avatar, Email: account.Email, Status: account.Status, DepartmentName: account.DepartmentName, RoleNames: account.RoleNames, CreateTime: formatTime(account.CreateTime)}
}

func accountToForm(account identitydomain.Account) gin.H {
	return gin.H{"id": strconv.FormatInt(account.ID, 10), "username": account.Username, "nickname": account.Nickname, "mobile": account.Mobile, "gender": account.Gender, "avatar": account.Avatar, "email": account.Email, "status": account.Status, "deptId": strconv.FormatInt(account.DepartmentID, 10), "roleIds": account.RoleIDs}
}

func queryInt(ctx *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(ctx.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func queryOptionalInt(ctx *gin.Context, key string) (int, bool) {
	raw := ctx.Query(key)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}
