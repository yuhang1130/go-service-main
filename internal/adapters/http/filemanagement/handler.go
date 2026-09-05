package filemanagement

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	fileapp "github.com/yuhang1130/go-service-main/internal/features/filemanagement/application"
	filedomain "github.com/yuhang1130/go-service-main/internal/features/filemanagement/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type Handler struct {
	service       *fileapp.Service
	publicBaseURL string
}

func NewHandler(service *fileapp.Service, publicBaseURL string) *Handler {
	return &Handler{service: service, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}

func (h *Handler) RegisterPublic(router *gin.RouterGroup) {
	router.GET("/files/content/*path", h.content)
}

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	files := router.Group("/files")
	files.POST("", middleware.RequirePermission("sys:file:create"), h.upload)
	files.POST("/batch", middleware.RequirePermission("sys:file:create"), h.uploadBatch)
	files.POST("/image", middleware.RequirePermission("sys:file:create"), h.uploadImage)
	files.DELETE("", middleware.RequirePermission("sys:file:delete"), h.delete)
}

func (h *Handler) upload(ctx *gin.Context) {
	header, err := ctx.FormFile("file")
	if err != nil {
		adminapi.Invalid(ctx, "请选择上传文件")
		return
	}
	stored, err := h.uploadFile(ctx, header, false)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, h.response(stored))
}

func (h *Handler) uploadBatch(ctx *gin.Context) {
	form, err := ctx.MultipartForm()
	if err != nil || len(form.File["files"]) == 0 {
		adminapi.Invalid(ctx, "请选择上传文件")
		return
	}
	const maximumFiles = 20
	headers := form.File["files"]
	if len(headers) > maximumFiles {
		adminapi.Invalid(ctx, "单次最多上传20个文件")
		return
	}
	storedFiles := make([]fileappFile, 0, len(headers))
	for _, header := range headers {
		stored, uploadErr := h.uploadFile(ctx, header, false)
		if uploadErr != nil {
			for _, uploaded := range storedFiles {
				_ = h.service.Delete(ctx.Request.Context(), uploaded.Key)
			}
			adminapi.Error(ctx, uploadErr)
			return
		}
		storedFiles = append(storedFiles, fileappFile{Key: stored.Key, Response: h.response(stored)})
	}
	result := make([]gin.H, len(storedFiles))
	for index, stored := range storedFiles {
		result[index] = stored.Response
	}
	adminapi.OK(ctx, result)
}

func (h *Handler) uploadImage(ctx *gin.Context) {
	header, err := ctx.FormFile("file")
	if err != nil {
		adminapi.Invalid(ctx, "请选择上传图片")
		return
	}
	stored, err := h.uploadFile(ctx, header, true)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, h.response(stored))
}

type fileappFile struct {
	Key      string
	Response gin.H
}

func (h *Handler) uploadFile(ctx *gin.Context, header *multipart.FileHeader, image bool) (filedomain.File, error) {
	file, err := header.Open()
	if err != nil {
		return filedomain.File{}, apperror.InvalidArgument("A0400", "上传文件无效", err)
	}
	defer file.Close()
	if image {
		return h.service.UploadImage(ctx.Request.Context(), header.Filename, header.Size, file)
	}
	return h.service.Upload(ctx.Request.Context(), header.Filename, header.Size, file)
}

func (h *Handler) response(stored filedomain.File) gin.H {
	url := h.publicBaseURL + "/api/v1/files/content/" + stored.Key
	return gin.H{"name": stored.Name, "url": url, "path": stored.Key, "size": stored.Size}
}

func (h *Handler) content(ctx *gin.Context) {
	key := strings.TrimPrefix(ctx.Param("path"), "/")
	reader, size, err := h.service.Open(ctx.Request.Context(), key)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	defer reader.Close()
	contentType := mime.TypeByExtension(path.Ext(key))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ctx.Header("Content-Type", contentType)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Content-Length", strconv.FormatInt(size, 10))
	ctx.Status(http.StatusOK)
	_, _ = io.Copy(ctx.Writer, reader)
}

func (h *Handler) delete(ctx *gin.Context) {
	filePath := ctx.Query("filePath")
	if filePath == "" {
		filePath = ctx.Query("path")
	}
	if err := h.service.Delete(ctx.Request.Context(), filePath); err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OKMessage(ctx, "删除成功")
}
