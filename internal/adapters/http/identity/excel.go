package identity

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	identitydomain "github.com/yuhang1130/go-service-main/internal/features/identity/domain"
)

const maximumImportBytes = 1 << 20

func (h *Handler) downloadTemplate(ctx *gin.Context) {
	workbook := excelize.NewFile()
	defer workbook.Close()
	sheet := workbook.GetSheetName(0)
	_ = workbook.SetSheetName(sheet, "用户导入模板")
	sheet = "用户导入模板"
	headers := []any{"用户名(*)", "昵称(*)", "手机号", "性别(0/1/2)", "邮箱", "角色编码或名称(逗号分隔)(*)", "部门编码或名称", "状态(启用/禁用)"}
	_ = workbook.SetSheetRow(sheet, "A1", &headers)
	example := []any{"zhangsan", "张三", "13800138000", 1, "zhangsan@example.com", "ADMIN", "DEFAULT", "启用"}
	_ = workbook.SetSheetRow(sheet, "A2", &example)
	setWorkbookLayout(workbook, sheet)
	h.writeWorkbook(ctx, workbook, "用户导入模板.xlsx")
}

func (h *Handler) exportUsers(ctx *gin.Context) {
	viewerID, _ := adminapi.AccountID(ctx)
	query, ok := userListQuery(ctx)
	if !ok {
		return
	}
	items, err := h.service.Export(ctx.Request.Context(), query, viewerID)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	workbook := excelize.NewFile()
	defer workbook.Close()
	sheet := workbook.GetSheetName(0)
	_ = workbook.SetSheetName(sheet, "用户列表")
	sheet = "用户列表"
	headers := []any{"用户ID", "用户名", "昵称", "手机号", "性别", "邮箱", "状态", "部门", "角色", "创建时间"}
	_ = workbook.SetSheetRow(sheet, "A1", &headers)
	for index, item := range items {
		row := exportRow(item)
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		_ = workbook.SetSheetRow(sheet, cell, &row)
	}
	setWorkbookLayout(workbook, sheet)
	h.writeWorkbook(ctx, workbook, "用户列表.xlsx")
}

func (h *Handler) importUsers(ctx *gin.Context) {
	header, err := ctx.FormFile("file")
	if err != nil || header.Size <= 0 || header.Size > maximumImportBytes || strings.ToLower(filepath.Ext(header.Filename)) != ".xlsx" {
		adminapi.Invalid(ctx, "请选择不超过1MB的xlsx文件")
		return
	}
	input, err := header.Open()
	if err != nil {
		adminapi.Invalid(ctx, "导入文件无效")
		return
	}
	defer input.Close()
	workbook, err := excelize.OpenReader(input)
	if err != nil {
		adminapi.Invalid(ctx, "Excel文件格式错误")
		return
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		adminapi.Invalid(ctx, "Excel文件没有工作表")
		return
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil || len(rows) < 2 {
		adminapi.Invalid(ctx, "Excel文件没有数据")
		return
	}
	candidates := make([]identityapp.ImportCandidate, 0, len(rows)-1)
	for index, row := range rows[1:] {
		if rowEmpty(row) {
			continue
		}
		candidates = append(candidates, parseCandidate(index+2, row))
	}
	actorID, _ := adminapi.AccountID(ctx)
	result, err := h.service.Import(ctx.Request.Context(), candidates, actorID)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, gin.H{"code": adminapi.CodeSuccess, "validCount": result.ValidCount, "invalidCount": result.InvalidCount, "messageList": result.Messages})
}

func (h *Handler) writeWorkbook(ctx *gin.Context, workbook *excelize.File, filename string) {
	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	ctx.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.QueryEscape(filename))
	ctx.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func setWorkbookLayout(workbook *excelize.File, sheet string) {
	_ = workbook.SetColWidth(sheet, "A", "A", 14)
	_ = workbook.SetColWidth(sheet, "B", "B", 18)
	_ = workbook.SetColWidth(sheet, "C", "C", 16)
	_ = workbook.SetColWidth(sheet, "D", "D", 14)
	_ = workbook.SetColWidth(sheet, "E", "J", 24)
	style, err := workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#E8EEF7"}, Pattern: 1}})
	if err == nil {
		_ = workbook.SetCellStyle(sheet, "A1", "J1", style)
	}
	_ = workbook.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

func exportRow(item identitydomain.Account) []any {
	gender := map[int]string{0: "未知", 1: "男", 2: "女"}[item.Gender]
	status := map[int]string{0: "禁用", 1: "启用"}[item.Status]
	return []any{strconv.FormatInt(item.ID, 10), item.Username, item.Nickname, item.Mobile, gender, item.Email, status, item.DepartmentName, item.RoleNames, formatTime(item.CreateTime)}
}

func parseCandidate(rowNumber int, row []string) identityapp.ImportCandidate {
	candidate := identityapp.ImportCandidate{Row: rowNumber, Username: cell(row, 0), Nickname: cell(row, 1), Mobile: cell(row, 2), Email: cell(row, 4), Department: cell(row, 6), Status: 1}
	gender := cell(row, 3)
	switch gender {
	case "", "0", "未知", "保密":
		candidate.Gender = 0
	case "1", "男":
		candidate.Gender = 1
	case "2", "女":
		candidate.Gender = 2
	default:
		candidate.ParseError = "性别无效"
	}
	roles := strings.ReplaceAll(cell(row, 5), "，", ",")
	for _, role := range strings.Split(roles, ",") {
		if role = strings.TrimSpace(role); role != "" {
			candidate.RoleTokens = append(candidate.RoleTokens, role)
		}
	}
	switch cell(row, 7) {
	case "", "1", "启用":
		candidate.Status = 1
	case "0", "禁用":
		candidate.Status = 0
	default:
		candidate.ParseError = "状态无效"
	}
	return candidate
}

func cell(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func rowEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
