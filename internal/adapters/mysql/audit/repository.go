package audit

import (
	"context"
	"strings"
	"time"

	auditapp "github.com/yuhang1130/go-service-main/internal/features/audit/application"
	auditdomain "github.com/yuhang1130/go-service-main/internal/features/audit/domain"
	"gorm.io/gorm"
)

type Repository struct{ database *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{database: database} }

type logRow struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	Module        string    `gorm:"column:module"`
	ActionType    string    `gorm:"column:action_type"`
	Title         string    `gorm:"column:title"`
	Content       string    `gorm:"column:content"`
	OperatorID    *int64    `gorm:"column:operator_id"`
	OperatorName  string    `gorm:"column:operator_name"`
	RequestURI    string    `gorm:"column:request_uri"`
	RequestMethod string    `gorm:"column:request_method"`
	IP            string    `gorm:"column:ip"`
	Region        string    `gorm:"column:region"`
	Device        string    `gorm:"column:device"`
	Browser       string    `gorm:"column:browser"`
	OS            string    `gorm:"column:os"`
	Status        int       `gorm:"column:status"`
	ExecutionTime int64     `gorm:"column:execution_time"`
	ErrorMessage  string    `gorm:"column:error_msg"`
	CreateTime    time.Time `gorm:"column:create_time"`
}

func (logRow) TableName() string { return "sys_log" }

func (r *Repository) Record(ctx context.Context, entry auditdomain.Entry) error {
	var operatorID *int64
	if entry.OperatorID > 0 {
		operatorID = &entry.OperatorID
	}
	row := logRow{Module: entry.Module, ActionType: entry.ActionType, Title: entry.Title, Content: entry.Content, OperatorID: operatorID, OperatorName: entry.OperatorName, RequestURI: entry.RequestURI, RequestMethod: entry.RequestMethod, IP: entry.IP, Region: entry.Region, Device: entry.Device, Browser: entry.Browser, OS: entry.OS, Status: entry.Status, ExecutionTime: entry.ExecutionTime, ErrorMessage: entry.ErrorMessage, CreateTime: entry.CreateTime}
	return r.database.WithContext(ctx).Create(&row).Error
}

func (r *Repository) List(ctx context.Context, query auditapp.Query) ([]auditdomain.Entry, int64, error) {
	database := r.database.WithContext(ctx).Table("sys_log l")
	if keyword := strings.TrimSpace(query.Keywords); keyword != "" {
		like := "%" + keyword + "%"
		database = database.Where("l.ip LIKE ? OR COALESCE(NULLIF(l.operator_name, ''), u.nickname, '') LIKE ? OR l.request_uri LIKE ?", like, like, like)
	}
	if query.CreatedFrom != nil {
		database = database.Where("l.create_time >= ?", *query.CreatedFrom)
	}
	if query.CreatedTo != nil {
		database = database.Where("l.create_time <= ?", *query.CreatedTo)
	}
	database = database.Joins("LEFT JOIN sys_user u ON u.id = l.operator_id")
	var total int64
	if err := database.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []logRow
	if err := database.Select("l.*, COALESCE(NULLIF(l.operator_name, ''), u.nickname, '') AS operator_name").Order("l.create_time DESC, l.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]auditdomain.Entry, len(rows))
	for index, row := range rows {
		items[index] = toDomain(row)
	}
	return items, total, nil
}

func (r *Repository) Daily(ctx context.Context, start, end time.Time) ([]auditdomain.DailyCount, error) {
	var rows []struct {
		Date       string `gorm:"column:date"`
		Operations int64  `gorm:"column:operations"`
		Operators  int64  `gorm:"column:operators"`
	}
	err := r.database.WithContext(ctx).Table("sys_log").Select("DATE_FORMAT(create_time, '%Y-%m-%d') AS date, COUNT(*) AS operations, COUNT(DISTINCT operator_id) AS operators").Where("create_time >= ? AND create_time < ?", start, end).Group("DATE_FORMAT(create_time, '%Y-%m-%d')").Order("date ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]auditdomain.DailyCount, len(rows))
	for index, row := range rows {
		result[index] = auditdomain.DailyCount{Date: row.Date, Operations: row.Operations, Operators: row.Operators}
	}
	return result, nil
}

func (r *Repository) Counts(ctx context.Context, now time.Time) (auditdomain.Counts, error) {
	location := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)
	var result auditdomain.Counts
	queries := []struct {
		destination *int64
		selectSQL   string
		where       string
		args        []any
	}{
		{&result.TodayOperations, "COUNT(*)", "create_time >= ? AND create_time < ?", []any{today, tomorrow}},
		{&result.YesterdayOperations, "COUNT(*)", "create_time >= ? AND create_time < ?", []any{yesterday, today}},
		{&result.TotalOperations, "COUNT(*)", "1 = 1", nil},
		{&result.TodayOperators, "COUNT(DISTINCT operator_id)", "create_time >= ? AND create_time < ?", []any{today, tomorrow}},
		{&result.YesterdayOperators, "COUNT(DISTINCT operator_id)", "create_time >= ? AND create_time < ?", []any{yesterday, today}},
		{&result.TotalOperators, "COUNT(DISTINCT operator_id)", "1 = 1", nil},
	}
	for _, query := range queries {
		if err := r.database.WithContext(ctx).Table("sys_log").Select(query.selectSQL).Where(query.where, query.args...).Scan(query.destination).Error; err != nil {
			return auditdomain.Counts{}, err
		}
	}
	return result, nil
}

func toDomain(row logRow) auditdomain.Entry {
	operatorID := int64(0)
	if row.OperatorID != nil {
		operatorID = *row.OperatorID
	}
	return auditdomain.Entry{ID: row.ID, Module: row.Module, ActionType: row.ActionType, Title: row.Title, Content: row.Content, OperatorID: operatorID, OperatorName: row.OperatorName, RequestURI: row.RequestURI, RequestMethod: row.RequestMethod, IP: row.IP, Region: row.Region, Device: row.Device, Browser: row.Browser, OS: row.OS, Status: row.Status, ExecutionTime: row.ExecutionTime, ErrorMessage: row.ErrorMessage, CreateTime: row.CreateTime}
}
