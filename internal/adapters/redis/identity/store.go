package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

var (
	ErrSessionNotFound    = identityapp.ErrSessionNotFound
	ErrRefreshTokenReused = identityapp.ErrRefreshTokenReused
)

var rotateSessionScript = redisclient.NewScript(`
local current = redis.call('GET', KEYS[1])
if not current or current ~= ARGV[1] then
  return 0
end

local old_access = redis.call('HGET', KEYS[4], 'access')
if old_access then
  redis.call('DEL', ARGV[7] .. old_access)
end

local family_ttl = redis.call('PTTL', KEYS[4])
if family_ttl <= 0 then
  family_ttl = tonumber(ARGV[6])
end
redis.call('SET', KEYS[1], ARGV[9], 'PX', family_ttl)
redis.call('SET', KEYS[2], ARGV[1], 'PX', ARGV[5])
redis.call('SET', KEYS[3], ARGV[1], 'PX', ARGV[6])
redis.call('HSET', KEYS[4], 'access', ARGV[2], 'refresh', ARGV[3], 'account', ARGV[4])
redis.call('PEXPIRE', KEYS[4], ARGV[6])
redis.call('SADD', KEYS[5], ARGV[8])
redis.call('PEXPIRE', KEYS[5], ARGV[6])
return 1
`)

var loginRateScript = redisclient.NewScript(`
local attempts = redis.call('INCR', KEYS[1])
if attempts == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
if attempts > tonumber(ARGV[2]) then
  local ttl = redis.call('PTTL', KEYS[1])
  if ttl < 1 then
    return 1
  end
  return ttl
end
return 0
`)

var revokeFamilyScript = redisclient.NewScript(`
local access = redis.call('HGET', KEYS[1], 'access')
local refresh = redis.call('HGET', KEYS[1], 'refresh')
if access then
  redis.call('DEL', ARGV[1] .. access)
end
if refresh then
  redis.call('DEL', ARGV[2] .. refresh)
end
redis.call('DEL', KEYS[1])
redis.call('SREM', KEYS[2], ARGV[3])
return 1
`)

var invalidateUserScript = redisclient.NewScript(`
local families = redis.call('SMEMBERS', KEYS[1])
for _, family_id in ipairs(families) do
  local family_key = ARGV[1] .. family_id
  local access = redis.call('HGET', family_key, 'access')
  local refresh = redis.call('HGET', family_key, 'refresh')
  if access then
    redis.call('DEL', ARGV[2] .. access)
  end
  if refresh then
    redis.call('DEL', ARGV[3] .. refresh)
  end
  redis.call('DEL', family_key)
end
redis.call('DEL', KEYS[1])
return #families
`)

type Store struct {
	client      *redisclient.Client
	accessTTL   time.Duration
	refreshTTL  time.Duration
	captchaTTL  time.Duration
	loginLimit  int64
	loginWindow time.Duration
}

type session struct {
	AccountID int64  `json:"accountId"`
	FamilyID  string `json:"familyId"`
	Used      bool   `json:"used,omitempty"`
}

func NewStore(client *redisclient.Client, cfg config.Identity) *Store {
	return &Store{
		client: client, accessTTL: cfg.AccessTokenTTL, refreshTTL: cfg.RefreshTokenTTL, captchaTTL: cfg.CaptchaTTL,
		loginLimit: int64(cfg.LoginRateLimit), loginWindow: cfg.LoginRateWindow,
	}
}

func (s *Store) AllowLogin(ctx context.Context, clientID string) (time.Duration, error) {
	retryAfterMillis, err := loginRateScript.Run(ctx, s.client, []string{loginRateKey(clientID)},
		s.loginWindow.Milliseconds(), s.loginLimit,
	).Int64()
	if err != nil {
		return 0, err
	}
	return time.Duration(retryAfterMillis) * time.Millisecond, nil
}

func (s *Store) Create(ctx context.Context, accountID int64) (identityapp.TokenPair, error) {
	familyID, err := randomToken()
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	return s.createFamily(ctx, accountID, familyID)
}

func (s *Store) createFamily(ctx context.Context, accountID int64, familyID string) (identityapp.TokenPair, error) {
	accessToken, err := randomToken()
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	refreshToken, err := randomToken()
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	value, err := json.Marshal(session{AccountID: accountID, FamilyID: familyID})
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	familyKey := sessionFamilyKey(familyID)
	_, err = s.client.TxPipelined(ctx, func(pipe redisclient.Pipeliner) error {
		pipe.Set(ctx, accessKey(accessToken), value, s.accessTTL)
		pipe.Set(ctx, refreshKey(refreshToken), value, s.refreshTTL)
		pipe.HSet(ctx, familyKey, "access", accessToken, "refresh", refreshToken, "account", accountID)
		pipe.Expire(ctx, familyKey, s.refreshTTL)
		userKey := userSessionsKey(accountID)
		pipe.SAdd(ctx, userKey, familyID)
		pipe.Expire(ctx, userKey, s.refreshTTL)
		return nil
	})
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	return identityapp.TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: "Bearer", ExpiresIn: int64(s.accessTTL.Seconds())}, nil
}

func (s *Store) Refresh(ctx context.Context, token string) (identityapp.TokenPair, error) {
	oldRefreshKey := refreshKey(token)
	value, err := s.client.Get(ctx, oldRefreshKey).Bytes()
	if errors.Is(err, redisclient.Nil) {
		return identityapp.TokenPair{}, ErrSessionNotFound
	}
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	var current session
	if err := json.Unmarshal(value, &current); err != nil {
		return identityapp.TokenPair{}, err
	}
	if current.Used {
		return identityapp.TokenPair{}, s.rejectReusedRefresh(ctx, current)
	}
	tombstone, err := json.Marshal(session{AccountID: current.AccountID, FamilyID: current.FamilyID, Used: true})
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	accessToken, err := randomToken()
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	refreshToken, err := randomToken()
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	familyKey := sessionFamilyKey(current.FamilyID)
	rotated, err := rotateSessionScript.Run(ctx, s.client, []string{
		oldRefreshKey,
		accessKey(accessToken),
		refreshKey(refreshToken),
		familyKey,
		userSessionsKey(current.AccountID),
	}, string(value), accessToken, refreshToken, current.AccountID,
		s.accessTTL.Milliseconds(), s.refreshTTL.Milliseconds(),
		accessKey(""), current.FamilyID, string(tombstone),
	).Int()
	if err != nil {
		return identityapp.TokenPair{}, err
	}
	if rotated != 1 {
		latest, getErr := s.client.Get(ctx, oldRefreshKey).Bytes()
		if getErr == nil {
			var observed session
			if unmarshalErr := json.Unmarshal(latest, &observed); unmarshalErr != nil {
				return identityapp.TokenPair{}, unmarshalErr
			}
			if observed.Used {
				return identityapp.TokenPair{}, s.rejectReusedRefresh(ctx, observed)
			}
		} else if !errors.Is(getErr, redisclient.Nil) {
			return identityapp.TokenPair{}, getErr
		}
		return identityapp.TokenPair{}, ErrSessionNotFound
	}
	return identityapp.TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: "Bearer", ExpiresIn: int64(s.accessTTL.Seconds())}, nil
}

func (s *Store) AccountID(ctx context.Context, token string) (int64, error) {
	value, err := s.client.Get(ctx, accessKey(token)).Bytes()
	if errors.Is(err, redisclient.Nil) {
		return 0, ErrSessionNotFound
	}
	if err != nil {
		return 0, err
	}
	var current session
	if err := json.Unmarshal(value, &current); err != nil {
		return 0, err
	}
	return current.AccountID, nil
}

func (s *Store) RevokeAccess(ctx context.Context, token string) error {
	value, err := s.client.Get(ctx, accessKey(token)).Bytes()
	if errors.Is(err, redisclient.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	var current session
	if err := json.Unmarshal(value, &current); err != nil {
		return err
	}
	return s.revokeFamily(ctx, current.AccountID, current.FamilyID)
}

func (s *Store) InvalidateUser(ctx context.Context, accountID int64) error {
	return invalidateUserScript.Run(ctx, s.client, []string{userSessionsKey(accountID)},
		sessionFamilyKey(""), accessKey(""), refreshKey(""),
	).Err()
}

func (s *Store) revokeFamily(ctx context.Context, accountID int64, familyID string) error {
	return revokeFamilyScript.Run(ctx, s.client, []string{
		sessionFamilyKey(familyID),
		userSessionsKey(accountID),
	}, accessKey(""), refreshKey(""), familyID).Err()
}

func (s *Store) rejectReusedRefresh(ctx context.Context, current session) error {
	if err := s.revokeFamily(ctx, current.AccountID, current.FamilyID); err != nil {
		return err
	}
	return ErrRefreshTokenReused
}

func (s *Store) Generate(ctx context.Context) (identityapp.Captcha, error) {
	id, err := randomToken()
	if err != nil {
		return identityapp.Captcha{}, err
	}
	code, err := randomDigits(4)
	if err != nil {
		return identityapp.Captcha{}, err
	}
	if err := s.client.Set(ctx, captchaKey(id), strings.ToLower(code), s.captchaTTL).Err(); err != nil {
		return identityapp.Captcha{}, err
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="140" height="44" viewBox="0 0 140 44"><rect width="140" height="44" rx="6" fill="#f0f8ff"/><text x="70" y="31" text-anchor="middle" font-family="monospace" font-size="28" font-weight="700" letter-spacing="8" fill="#2563eb">%s</text></svg>`, code)
	return identityapp.Captcha{ID: id, Image: "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))}, nil
}

func (s *Store) Verify(ctx context.Context, id, code string) (bool, error) {
	consume := redisclient.NewScript(`local value = redis.call('GET', KEYS[1]); if not value then return false end; redis.call('DEL', KEYS[1]); return value`)
	value, err := consume.Run(ctx, s.client, []string{captchaKey(id)}).Text()
	if errors.Is(err, redisclient.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(value, strings.TrimSpace(code)), nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func randomDigits(length int) (string, error) {
	const digits = "23456789"
	value := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for index := range value {
		value[index] = digits[int(random[index])%len(digits)]
	}
	return string(value), nil
}

func accessKey(token string) string     { return "identity:access:" + token }
func refreshKey(token string) string    { return "identity:refresh:" + token }
func sessionFamilyKey(id string) string { return "identity:session:" + id }
func userSessionsKey(accountID int64) string {
	return fmt.Sprintf("identity:user:%d:sessions", accountID)
}
func captchaKey(id string) string { return "identity:captcha:" + id }
func loginRateKey(clientID string) string {
	digest := sha256.Sum256([]byte(clientID))
	return fmt.Sprintf("identity:login-rate:%x", digest)
}
