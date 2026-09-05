package application

import (
	"context"
	"time"

	"github.com/yuhang1130/go-service-main/internal/features/audit/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type Query struct {
	Page, PageSize int
	Keywords       string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
}

type Trend struct {
	Dates         []string
	OperationList []int64
	OperatorList  []int64
}

type Overview struct {
	TodayOperatorCount  int64
	TotalOperatorCount  int64
	OperatorGrowthRate  float64
	TodayOperationCount int64
	TotalOperationCount int64
	OperationGrowthRate float64
}

type Repository interface {
	Record(context.Context, domain.Entry) error
	List(context.Context, Query) ([]domain.Entry, int64, error)
	Daily(context.Context, time.Time, time.Time) ([]domain.DailyCount, error)
	Counts(context.Context, time.Time) (domain.Counts, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Record(ctx context.Context, entry domain.Entry) error {
	return s.repository.Record(ctx, entry)
}

func (s *Service) List(ctx context.Context, query Query) ([]domain.Entry, int64, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 10
	}
	if query.PageSize > 200 {
		query.PageSize = 200
	}
	items, total, err := s.repository.List(ctx, query)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) Trend(ctx context.Context, start, end time.Time) (Trend, error) {
	if start.After(end) {
		return Trend{}, apperror.InvalidArgument("A0400", "开始日期不能晚于结束日期", nil)
	}
	if end.Sub(start) > 90*24*time.Hour {
		return Trend{}, apperror.InvalidArgument("A0400", "查询范围不能超过90天", nil)
	}
	counts, err := s.repository.Daily(ctx, start, end.AddDate(0, 0, 1))
	if err != nil {
		return Trend{}, apperror.Internal(err)
	}
	indexed := make(map[string]domain.DailyCount, len(counts))
	for _, count := range counts {
		indexed[count.Date] = count
	}
	result := Trend{}
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		key := date.Format("2006-01-02")
		count := indexed[key]
		result.Dates = append(result.Dates, key)
		result.OperationList = append(result.OperationList, count.Operations)
		result.OperatorList = append(result.OperatorList, count.Operators)
	}
	return result, nil
}

func (s *Service) Overview(ctx context.Context, now time.Time) (Overview, error) {
	counts, err := s.repository.Counts(ctx, now)
	if err != nil {
		return Overview{}, apperror.Internal(err)
	}
	return Overview{TodayOperatorCount: counts.TodayOperators, TotalOperatorCount: counts.TotalOperators, OperatorGrowthRate: growth(counts.TodayOperators, counts.YesterdayOperators), TodayOperationCount: counts.TodayOperations, TotalOperationCount: counts.TotalOperations, OperationGrowthRate: growth(counts.TodayOperations, counts.YesterdayOperations)}, nil
}

func growth(current, previous int64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return float64(current-previous) * 100 / float64(previous)
}
