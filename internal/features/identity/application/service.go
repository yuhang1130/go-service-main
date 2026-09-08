package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
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

type Repository interface {
	Count(context.Context) (int64, error)
	GetByUsername(context.Context, string) (domain.Account, error)
	Get(context.Context, int64) (domain.Account, error)
	List(context.Context, ListQuery, AccountScope) ([]domain.Account, int64, error)
	Export(context.Context, ListQuery, AccountScope, int) ([]domain.Account, error)
	Options(context.Context, AccountScope) ([]domain.Account, error)
	UsernameExists(context.Context, string, int64) (bool, error)
	ExistingUsernames(context.Context, []string) (map[string]struct{}, error)
	Save(context.Context, domain.Account, []int64, int64) error
	Delete(context.Context, []int64, int64) error
	SetStatus(context.Context, int64, int, int64) error
	SetPassword(context.Context, int64, string, int64) error
	SetProfile(context.Context, int64, ProfileCommand) error
	Bootstrap(context.Context, domain.Account) (bool, error)
	IsRoot(context.Context, int64) (bool, error)
	ImportReferences(context.Context) (ImportReferences, error)
	Import(context.Context, []domain.Account, int64) error
}

type Sessions interface {
	Create(context.Context, int64) (TokenPair, error)
	Refresh(context.Context, string) (TokenPair, error)
	AccountID(context.Context, string) (int64, error)
	RevokeAccess(context.Context, string) error
	InvalidateUser(context.Context, int64) error
}

type Captchas interface {
	Generate(context.Context) (Captcha, error)
	Verify(context.Context, string, string) (bool, error)
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Compare(string, string) bool
}

type Authorizer interface {
	Authorization(context.Context, int64) (Authorization, error)
	Scope(context.Context, int64) (AccountScope, error)
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
		return false, apperror.InvalidArgument(apperror.CodeInvalidArgument, "bootstrap account requires a username and a password of at least 8 characters", nil)
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
	created, err := s.repository.Bootstrap(ctx, account)
	if err != nil {
		return false, mapConflict(err, "初始化用户名已存在")
	}
	return created, nil
}

func (s *Service) Captcha(ctx context.Context) (Captcha, error) {
	captcha, err := s.captchas.Generate(ctx)
	if err != nil {
		return Captcha{}, apperror.Internal(err)
	}
	return captcha, nil
}

func (s *Service) Login(ctx context.Context, command LoginCommand) (TokenPair, error) {
	if strings.TrimSpace(command.Username) == "" || command.Password == "" || command.CaptchaID == "" || command.CaptchaCode == "" {
		return TokenPair{}, apperror.InvalidArgument(apperror.CodeInvalidArgument, "用户名、密码和验证码不能为空", nil)
	}
	valid, err := s.captchas.Verify(ctx, command.CaptchaID, command.CaptchaCode)
	if err != nil {
		return TokenPair{}, apperror.Internal(err)
	}
	if !valid {
		return TokenPair{}, apperror.InvalidArgument(apperror.CodeInvalidArgument, "验证码错误或已过期", nil)
	}
	account, err := s.repository.GetByUsername(ctx, strings.TrimSpace(command.Username))
	if errors.Is(err, ErrNotFound) {
		return TokenPair{}, apperror.Unauthorized(apperror.CodeInvalidCredentials, "用户名或密码错误")
	}
	if err != nil {
		return TokenPair{}, apperror.Internal(err)
	}
	if !s.passwords.Compare(account.Password, command.Password) {
		return TokenPair{}, apperror.Unauthorized(apperror.CodeInvalidCredentials, "用户名或密码错误")
	}
	if account.Status != 1 {
		return TokenPair{}, apperror.Forbidden(apperror.CodeForbidden, "用户已被禁用")
	}
	tokens, err := s.sessions.Create(ctx, account.ID)
	if err != nil {
		return TokenPair{}, apperror.Internal(err)
	}
	return tokens, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return TokenPair{}, apperror.InvalidArgument(apperror.CodeInvalidArgument, "刷新令牌不能为空", nil)
	}
	tokens, err := s.sessions.Refresh(ctx, refreshToken)
	if err != nil {
		if !errors.Is(err, ErrSessionNotFound) && !errors.Is(err, ErrRefreshTokenReused) {
			return TokenPair{}, apperror.Internal(err)
		}
		return TokenPair{}, apperror.Unauthorized(apperror.CodeInvalidRefreshToken, "刷新令牌无效或已过期")
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

func (s *Service) VerifyAccount(ctx context.Context, accessToken string) (domain.Account, Authorization, error) {
	accountID, err := s.sessions.AccountID(ctx, accessToken)
	if err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			return domain.Account{}, Authorization{}, apperror.Internal(err)
		}
		return domain.Account{}, Authorization{}, apperror.Unauthorized(apperror.CodeInvalidAccessToken, "访问令牌无效或已过期")
	}
	account, err := s.repository.Get(ctx, accountID)
	if errors.Is(err, ErrNotFound) || (err == nil && account.Status != 1) {
		_ = s.sessions.InvalidateUser(ctx, accountID)
		return domain.Account{}, Authorization{}, apperror.Unauthorized(apperror.CodeInvalidAccessToken, "访问令牌无效或已过期")
	}
	if err != nil {
		return domain.Account{}, Authorization{}, apperror.Internal(err)
	}
	authorization, err := s.authorizer.Authorization(ctx, accountID)
	if err != nil {
		return domain.Account{}, Authorization{}, err
	}
	return account, authorization, nil
}

func (s *Service) Current(ctx context.Context, accountID int64) (CurrentUser, error) {
	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return CurrentUser{}, mapError(err, "用户不存在")
	}
	authorization, err := s.authorizer.Authorization(ctx, accountID)
	if err != nil {
		return CurrentUser{}, err
	}
	return CurrentUser{UserID: account.ID, Username: account.Username, Nickname: account.Nickname, Avatar: account.Avatar, Roles: authorization.Roles, Permissions: authorization.Permissions}, nil
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
		return apperror.InvalidArgument(apperror.CodeInvalidArgument, "用户名、昵称、性别、状态或角色无效", err)
	}
	exists, err := s.repository.UsernameExists(ctx, account.Username, account.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if exists {
		return apperror.Conflict(apperror.CodeConflict, "用户名已存在")
	}
	if account.ID == 0 {
		if len(s.defaultPassword) < 8 {
			return apperror.InvalidArgument(apperror.CodeInvalidArgument, "未配置新用户初始密码", nil)
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
				return apperror.Forbidden(apperror.CodeForbidden, "不能修改超级管理员用户名")
			}
		}
		account.Password = current.Password
	}
	if err := s.repository.Save(ctx, account, account.RoleIDs, actorID); err != nil {
		if errors.Is(err, ErrInvalidAssociation) {
			return apperror.InvalidArgument(apperror.CodeInvalidArgument, "部门或角色不存在、已禁用", nil)
		}
		return mapConflict(err, "用户名或用户关联关系已存在")
	}
	if account.ID != 0 {
		_ = s.sessions.InvalidateUser(ctx, account.ID)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, ids []int64, actorID int64) error {
	if len(ids) == 0 {
		return apperror.InvalidArgument(apperror.CodeInvalidArgument, "用户ID不能为空", nil)
	}
	for _, id := range ids {
		if id == actorID {
			return apperror.Forbidden(apperror.CodeForbidden, "不能删除当前登录用户")
		}
		root, err := s.repository.IsRoot(ctx, id)
		if err != nil {
			return apperror.Internal(err)
		}
		if root {
			return apperror.Forbidden(apperror.CodeForbidden, "不能删除超级管理员")
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
		return apperror.InvalidArgument(apperror.CodeInvalidArgument, "状态值无效", nil)
	}
	if id == actorID && status == 0 {
		return apperror.Forbidden(apperror.CodeForbidden, "不能禁用当前登录用户")
	}
	root, err := s.repository.IsRoot(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if root && status == 0 {
		return apperror.Forbidden(apperror.CodeForbidden, "不能禁用超级管理员")
	}
	if err := s.repository.SetStatus(ctx, id, status, actorID); err != nil {
		return mapError(err, "用户不存在")
	}
	_ = s.sessions.InvalidateUser(ctx, id)
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, id int64, password string, actorID int64) error {
	if len(password) < 8 {
		return apperror.InvalidArgument(apperror.CodeInvalidArgument, "密码至少需要8个字符", nil)
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
		return apperror.InvalidArgument(apperror.CodeInvalidArgument, "昵称或性别无效", nil)
	}
	if err := s.repository.SetProfile(ctx, id, command); err != nil {
		return mapError(err, "用户不存在")
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, id int64, command PasswordCommand) error {
	if command.NewPassword != command.ConfirmPassword || len(command.NewPassword) < 8 {
		return apperror.InvalidArgument(apperror.CodeInvalidArgument, "两次密码不一致或密码少于8个字符", nil)
	}
	account, err := s.repository.Get(ctx, id)
	if err != nil {
		return mapError(err, "用户不存在")
	}
	if !s.passwords.Compare(account.Password, command.OldPassword) {
		return apperror.InvalidArgument(apperror.CodeInvalidArgument, "当前密码错误", nil)
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
		return apperror.NotFound(apperror.CodeNotFound, message)
	}
	return apperror.Internal(err)
}

func mapConflict(err error, message string) error {
	if errors.Is(err, persistence.ErrConflict) {
		return apperror.Conflict(apperror.CodeConflict, message)
	}
	return apperror.Internal(err)
}
