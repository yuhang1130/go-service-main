package application

import (
	"context"
	"strconv"
	"strings"

	"github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type ImportCandidate struct {
	Row        int
	Username   string
	Nickname   string
	Mobile     string
	Gender     int
	Email      string
	RoleTokens []string
	Department string
	Status     int
	ParseError string
}

type ImportReferences struct {
	Roles       map[string]int64
	Departments map[string]int64
}

type ImportResult struct {
	ValidCount   int
	InvalidCount int
	Messages     []string
}

func (s *Service) Import(ctx context.Context, candidates []ImportCandidate, actorID int64) (ImportResult, error) {
	result := ImportResult{Messages: []string{}}
	if len(candidates) == 0 || len(candidates) > 1000 {
		return result, apperror.InvalidArgument(apperror.CodeInvalidArgument, "导入文件没有数据或超过1000行", nil)
	}
	if len(s.defaultPassword) < 8 {
		return result, apperror.InvalidArgument(apperror.CodeInvalidArgument, "未配置新用户初始密码", nil)
	}
	references, err := s.repository.ImportReferences(ctx)
	if err != nil {
		return result, apperror.Internal(err)
	}
	password, err := s.passwords.Hash(s.defaultPassword)
	if err != nil {
		return result, apperror.Internal(err)
	}
	usernames := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if username := strings.TrimSpace(candidate.Username); username != "" {
			usernames = append(usernames, username)
		}
	}
	existing, err := s.repository.ExistingUsernames(ctx, usernames)
	if err != nil {
		return result, apperror.Internal(err)
	}
	seen := make(map[string]struct{}, len(candidates))
	valid := make([]domain.Account, 0, len(candidates))
	for _, candidate := range candidates {
		account, message := prepareImport(candidate, references, password, seen, existing)
		if message != "" {
			result.InvalidCount++
			result.Messages = append(result.Messages, message)
			continue
		}
		seen[account.Username] = struct{}{}
		valid = append(valid, account)
	}
	if len(valid) > 0 {
		if err := s.repository.Import(ctx, valid, actorID); err != nil {
			return result, mapConflict(err, "导入用户中存在重复用户名")
		}
	}
	result.ValidCount = len(valid)
	return result, nil
}

func prepareImport(candidate ImportCandidate, references ImportReferences, password string, seen, existing map[string]struct{}) (domain.Account, string) {
	prefix := "第" + strconv.Itoa(candidate.Row) + "行: "
	if candidate.ParseError != "" {
		return domain.Account{}, prefix + candidate.ParseError
	}
	account := domain.Account{
		Username: strings.TrimSpace(candidate.Username), Nickname: strings.TrimSpace(candidate.Nickname),
		Mobile: strings.TrimSpace(candidate.Mobile), Gender: candidate.Gender,
		Email: strings.TrimSpace(candidate.Email), Status: candidate.Status, Password: password,
	}
	for _, token := range candidate.RoleTokens {
		if id, ok := references.Roles[strings.TrimSpace(token)]; ok {
			account.RoleIDs = append(account.RoleIDs, id)
		}
	}
	if candidate.Department != "" {
		account.DepartmentID = references.Departments[strings.TrimSpace(candidate.Department)]
		if account.DepartmentID == 0 {
			return domain.Account{}, prefix + "部门不存在"
		}
	}
	if err := account.Validate(); err != nil || len(account.RoleIDs) == 0 {
		return domain.Account{}, prefix + "用户名、昵称、性别、状态或角色无效"
	}
	if _, duplicate := seen[account.Username]; duplicate {
		return domain.Account{}, prefix + "用户名在文件内重复"
	}
	if _, exists := existing[account.Username]; exists {
		return domain.Account{}, prefix + "用户名已存在"
	}
	return account, ""
}
