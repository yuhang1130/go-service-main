package application

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	accessdomain "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	"github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrSessionNotFound = errors.New("session not found")
)

type LoginCommand struct {
	Username    string
	Password    string
	CaptchaID   string
	CaptchaCode string
}

type ListQuery struct {
	Page         int
	PageSize     int
	Keywords     string
	Status       *int
	DepartmentID *int64
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
}

type SaveCommand struct {
	ID           int64
	Username     string
	Nickname     string
	Mobile       string
	Gender       int
	Avatar       string
	Email        string
	Status       int
	DepartmentID int64
	RoleIDs      []int64
}

type ProfileCommand struct {
	Nickname string
	Avatar   string
	Gender   int
}

type PasswordCommand struct {
	OldPassword     string
	NewPassword     string
	ConfirmPassword string
}

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

type Repository interface {
	Count(context.Context) (int64, error)
	GetByUsername(context.Context, string) (domain.Account, error)
	Get(context.Context, int64) (domain.Account, error)
	List(context.Context, ListQuery, accessdomain.AccountScope) ([]domain.Account, int64, error)
	Export(context.Context, ListQuery, accessdomain.AccountScope, int) ([]domain.Account, error)
	Options(context.Context, accessdomain.AccountScope) ([]domain.Account, error)
	UsernameExists(context.Context, string, int64) (bool, error)
	Save(context.Context, domain.Account, []int64, int64) error
	Delete(context.Context, []int64, int64) error
	SetStatus(context.Context, int64, int, int64) error
	SetPassword(context.Context, int64, string, int64) error
	SetProfile(context.Context, int64, ProfileCommand) error
	Bootstrap(context.Context, domain.Account, string) (bool, error)
	IsRoot(context.Context, int64) (bool, error)
	ImportReferences(context.Context) (ImportReferences, error)
	Import(context.Context, []domain.Account, int64) error
}

type Sessions interface {
	Create(context.Context, int64) (domain.TokenPair, error)
	Refresh(context.Context, string) (domain.TokenPair, error)
	AccountID(context.Context, string) (int64, error)
	RevokeAccess(context.Context, string) error
	InvalidateUser(context.Context, int64) error
}

type Captchas interface {
	Generate(context.Context) (domain.Captcha, error)
	Verify(context.Context, string, string) (bool, error)
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Compare(string, string) bool
}

type Authorizer interface {
	Authorization(context.Context, int64) (accessdomain.Authorization, error)
	Scope(context.Context, int64) (accessdomain.AccountScope, error)
}

type Service struct {
	repository      Repository
	sessions        Sessions
	captchas        Captchas
	passwords       PasswordHasher
	authorizer      Authorizer
	defaultPassword string
}

func NewService(repository Repository, sessions Sessions, captchas Captchas, passwords PasswordHasher, authorizer Authorizer, defaultPassword string) *Service {
	return &Service{repository: repository, sessions: sessions, captchas: captchas, passwords: passwords, authorizer: authorizer, defaultPassword: defaultPassword}
}

func (s *Service) Bootstrap(ctx context.Context, username, password string) (bool, error) {
	if strings.TrimSpace(username) == "" && password == "" {
		return false, nil
	}
	if strings.TrimSpace(username) == "" || len(password) < 8 {
		return false, apperror.InvalidArgument("A0400", "bootstrap account requires a username and a password of at least 8 characters", nil)
	}
	count, err := s.repository.Count(ctx)
	if err != nil {
		return false, apperror.Internal(err)
	}
	if count != 0 {
		return false, nil
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return false, apperror.Internal(err)
	}
	account := domain.Account{Username: strings.TrimSpace(username), Nickname: "超级管理员", Gender: 0, Password: hash, DepartmentID: 1, Status: 1}
	created, err := s.repository.Bootstrap(ctx, account, accessdomain.RootRoleCode)
	if err != nil {
		return false, mapConflict(err, "初始化用户名已存在")
	}
	return created, nil
}

func (s *Service) Captcha(ctx context.Context) (domain.Captcha, error) {
	captcha, err := s.captchas.Generate(ctx)
	if err != nil {
		return domain.Captcha{}, apperror.Internal(err)
	}
	return captcha, nil
}

func (s *Service) Login(ctx context.Context, command LoginCommand) (domain.TokenPair, error) {
	if strings.TrimSpace(command.Username) == "" || command.Password == "" || command.CaptchaID == "" || command.CaptchaCode == "" {
		return domain.TokenPair{}, apperror.InvalidArgument("A0400", "用户名、密码和验证码不能为空", nil)
	}
	valid, err := s.captchas.Verify(ctx, command.CaptchaID, command.CaptchaCode)
	if err != nil {
		return domain.TokenPair{}, apperror.Internal(err)
	}
	if !valid {
		return domain.TokenPair{}, apperror.InvalidArgument("A0400", "验证码错误或已过期", nil)
	}
	account, err := s.repository.GetByUsername(ctx, strings.TrimSpace(command.Username))
	if errors.Is(err, ErrNotFound) {
		return domain.TokenPair{}, apperror.Unauthorized("A0210", "用户名或密码错误")
	}
	if err != nil {
		return domain.TokenPair{}, apperror.Internal(err)
	}
	if !s.passwords.Compare(account.Password, command.Password) {
		return domain.TokenPair{}, apperror.Unauthorized("A0210", "用户名或密码错误")
	}
	if account.Status != 1 {
		return domain.TokenPair{}, apperror.Forbidden("A0300", "用户已被禁用")
	}
	tokens, err := s.sessions.Create(ctx, account.ID)
	if err != nil {
		return domain.TokenPair{}, apperror.Internal(err)
	}
	return tokens, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return domain.TokenPair{}, apperror.InvalidArgument("A0400", "刷新令牌不能为空", nil)
	}
	tokens, err := s.sessions.Refresh(ctx, refreshToken)
	if err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			return domain.TokenPair{}, apperror.Internal(err)
		}
		return domain.TokenPair{}, apperror.Unauthorized("A0231", "刷新令牌无效或已过期")
	}
	return tokens, nil
}

func (s *Service) Logout(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return nil
	}
	if err := s.sessions.RevokeAccess(ctx, accessToken); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *Service) VerifyAccount(ctx context.Context, accessToken string) (domain.Account, accessdomain.Authorization, error) {
	accountID, err := s.sessions.AccountID(ctx, accessToken)
	if err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			return domain.Account{}, accessdomain.Authorization{}, apperror.Internal(err)
		}
		return domain.Account{}, accessdomain.Authorization{}, apperror.Unauthorized("A0230", "访问令牌无效或已过期")
	}
	account, err := s.repository.Get(ctx, accountID)
	if errors.Is(err, ErrNotFound) || (err == nil && account.Status != 1) {
		_ = s.sessions.InvalidateUser(ctx, accountID)
		return domain.Account{}, accessdomain.Authorization{}, apperror.Unauthorized("A0230", "访问令牌无效或已过期")
	}
	if err != nil {
		return domain.Account{}, accessdomain.Authorization{}, apperror.Internal(err)
	}
	authorization, err := s.authorizer.Authorization(ctx, accountID)
	if err != nil {
		return domain.Account{}, accessdomain.Authorization{}, err
	}
	return account, authorization, nil
}

func (s *Service) Current(ctx context.Context, accountID int64) (domain.Current, error) {
	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return domain.Current{}, mapError(err, "用户不存在")
	}
	authorization, err := s.authorizer.Authorization(ctx, accountID)
	if err != nil {
		return domain.Current{}, err
	}
	return domain.Current{UserID: account.ID, Username: account.Username, Nickname: account.Nickname, Avatar: account.Avatar, Roles: authorization.Roles, Permissions: authorization.Permissions}, nil
}

func (s *Service) List(ctx context.Context, query ListQuery, viewerID int64) ([]domain.Account, int64, error) {
	normalizePage(&query)
	scope, err := s.authorizer.Scope(ctx, viewerID)
	if err != nil {
		return nil, 0, err
	}
	items, total, err := s.repository.List(ctx, query, scope)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) Export(ctx context.Context, query ListQuery, viewerID int64) ([]domain.Account, error) {
	scope, err := s.authorizer.Scope(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	items, err := s.repository.Export(ctx, query, scope, 10000)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

func (s *Service) Import(ctx context.Context, candidates []ImportCandidate, actorID int64) (ImportResult, error) {
	result := ImportResult{Messages: []string{}}
	if len(candidates) == 0 || len(candidates) > 1000 {
		return result, apperror.InvalidArgument("A0400", "导入文件没有数据或超过1000行", nil)
	}
	if len(s.defaultPassword) < 8 {
		return result, apperror.InvalidArgument("A0400", "未配置新用户初始密码", nil)
	}
	references, err := s.repository.ImportReferences(ctx)
	if err != nil {
		return result, apperror.Internal(err)
	}
	password, err := s.passwords.Hash(s.defaultPassword)
	if err != nil {
		return result, apperror.Internal(err)
	}
	seen := make(map[string]struct{}, len(candidates))
	valid := make([]domain.Account, 0, len(candidates))
	for _, candidate := range candidates {
		account, message := s.prepareImport(ctx, candidate, references, password, seen)
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

func (s *Service) prepareImport(ctx context.Context, candidate ImportCandidate, references ImportReferences, password string, seen map[string]struct{}) (domain.Account, string) {
	prefix := "第" + strconv.Itoa(candidate.Row) + "行: "
	if candidate.ParseError != "" {
		return domain.Account{}, prefix + candidate.ParseError
	}
	account := domain.Account{Username: strings.TrimSpace(candidate.Username), Nickname: strings.TrimSpace(candidate.Nickname), Mobile: strings.TrimSpace(candidate.Mobile), Gender: candidate.Gender, Email: strings.TrimSpace(candidate.Email), Status: candidate.Status, Password: password}
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
	exists, err := s.repository.UsernameExists(ctx, account.Username, 0)
	if err != nil {
		return domain.Account{}, prefix + "用户名校验失败"
	}
	if exists {
		return domain.Account{}, prefix + "用户名已存在"
	}
	return account, ""
}

func (s *Service) Options(ctx context.Context, viewerID int64) ([]domain.Account, error) {
	scope, err := s.authorizer.Scope(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	items, err := s.repository.Options(ctx, scope)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, id int64) (domain.Account, error) {
	account, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Account{}, mapError(err, "用户不存在")
	}
	return account, nil
}

func (s *Service) Save(ctx context.Context, command SaveCommand, actorID int64) error {
	account := domain.Account{ID: command.ID, Username: strings.TrimSpace(command.Username), Nickname: strings.TrimSpace(command.Nickname), Mobile: strings.TrimSpace(command.Mobile), Gender: command.Gender, Avatar: strings.TrimSpace(command.Avatar), Email: strings.TrimSpace(command.Email), Status: command.Status, DepartmentID: command.DepartmentID, RoleIDs: command.RoleIDs}
	if err := account.Validate(); err != nil || len(account.RoleIDs) == 0 {
		return apperror.InvalidArgument("A0400", "用户名、昵称、性别、状态或角色无效", err)
	}
	exists, err := s.repository.UsernameExists(ctx, account.Username, account.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if exists {
		return apperror.Conflict("A0409", "用户名已存在")
	}
	if account.ID == 0 {
		if len(s.defaultPassword) < 8 {
			return apperror.InvalidArgument("A0400", "未配置新用户初始密码", nil)
		}
		account.Password, err = s.passwords.Hash(s.defaultPassword)
		if err != nil {
			return apperror.Internal(err)
		}
	} else {
		current, err := s.repository.Get(ctx, account.ID)
		if err != nil {
			return mapError(err, "用户不存在")
		}
		if current.Username != account.Username {
			root, rootErr := s.repository.IsRoot(ctx, account.ID)
			if rootErr != nil {
				return apperror.Internal(rootErr)
			}
			if root {
				return apperror.Forbidden("A0300", "不能修改超级管理员用户名")
			}
		}
		account.Password = current.Password
	}
	if err := s.repository.Save(ctx, account, account.RoleIDs, actorID); err != nil {
		return mapConflict(err, "用户名或用户关联关系已存在")
	}
	if account.ID != 0 {
		_ = s.sessions.InvalidateUser(ctx, account.ID)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, ids []int64, actorID int64) error {
	if len(ids) == 0 {
		return apperror.InvalidArgument("A0400", "用户ID不能为空", nil)
	}
	for _, id := range ids {
		if id == actorID {
			return apperror.Forbidden("A0300", "不能删除当前登录用户")
		}
		root, err := s.repository.IsRoot(ctx, id)
		if err != nil {
			return apperror.Internal(err)
		}
		if root {
			return apperror.Forbidden("A0300", "不能删除超级管理员")
		}
	}
	if err := s.repository.Delete(ctx, ids, actorID); err != nil {
		return mapError(err, "用户不存在")
	}
	for _, id := range ids {
		_ = s.sessions.InvalidateUser(ctx, id)
	}
	return nil
}

func (s *Service) SetStatus(ctx context.Context, id int64, status int, actorID int64) error {
	if status != 0 && status != 1 {
		return apperror.InvalidArgument("A0400", "状态值无效", nil)
	}
	if id == actorID && status == 0 {
		return apperror.Forbidden("A0300", "不能禁用当前登录用户")
	}
	root, err := s.repository.IsRoot(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if root && status == 0 {
		return apperror.Forbidden("A0300", "不能禁用超级管理员")
	}
	if err := s.repository.SetStatus(ctx, id, status, actorID); err != nil {
		return mapError(err, "用户不存在")
	}
	_ = s.sessions.InvalidateUser(ctx, id)
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, id int64, password string, actorID int64) error {
	if len(password) < 8 {
		return apperror.InvalidArgument("A0400", "密码至少需要8个字符", nil)
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return apperror.Internal(err)
	}
	if err := s.repository.SetPassword(ctx, id, hash, actorID); err != nil {
		return mapError(err, "用户不存在")
	}
	_ = s.sessions.InvalidateUser(ctx, id)
	return nil
}

func (s *Service) Profile(ctx context.Context, id int64) (domain.Account, error) {
	return s.Get(ctx, id)
}

func (s *Service) UpdateProfile(ctx context.Context, id int64, command ProfileCommand) error {
	if strings.TrimSpace(command.Nickname) == "" || command.Gender < 0 || command.Gender > 2 {
		return apperror.InvalidArgument("A0400", "昵称或性别无效", nil)
	}
	if err := s.repository.SetProfile(ctx, id, command); err != nil {
		return mapError(err, "用户不存在")
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, id int64, command PasswordCommand) error {
	if command.NewPassword != command.ConfirmPassword || len(command.NewPassword) < 8 {
		return apperror.InvalidArgument("A0400", "两次密码不一致或密码少于8个字符", nil)
	}
	account, err := s.repository.Get(ctx, id)
	if err != nil {
		return mapError(err, "用户不存在")
	}
	if !s.passwords.Compare(account.Password, command.OldPassword) {
		return apperror.InvalidArgument("A0400", "当前密码错误", nil)
	}
	return s.ResetPassword(ctx, id, command.NewPassword, id)
}

func normalizePage(query *ListQuery) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 200 {
		query.PageSize = 10
	}
}

func mapError(err error, message string) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("A0404", message)
	}
	return apperror.Internal(err)
}

func mapConflict(err error, message string) error {
	if errors.Is(err, persistence.ErrConflict) {
		return apperror.Conflict("A0409", message)
	}
	return apperror.Internal(err)
}
