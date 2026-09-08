package middleware

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ClientRateLimiter struct {
	limit   rate.Limit
	burst   int
	ttl     time.Duration
	now     func() time.Time
	mu      sync.Mutex
	clients map[string]*clientLimiter
	cleaned time.Time
}

func NewClientRateLimiter(cfg config.HTTPRateLimit) *ClientRateLimiter {
	return &ClientRateLimiter{
		limit:   rate.Limit(cfg.RequestsPerSecond),
		burst:   cfg.Burst,
		ttl:     cfg.ClientTTL,
		now:     time.Now,
		clients: make(map[string]*clientLimiter),
	}
}

func (l *ClientRateLimiter) Allow(key string) (bool, time.Duration) {
	now := l.now()
	limiter := l.forKey(strings.TrimSpace(key), now)
	reservation := limiter.ReserveN(now, 1)
	if !reservation.OK() {
		return false, time.Second
	}
	delay := reservation.DelayFrom(now)
	if delay <= 0 {
		return true, 0
	}
	reservation.CancelAt(now)
	return false, delay
}

func (l *ClientRateLimiter) forKey(key string, now time.Time) *rate.Limiter {
	if key == "" {
		key = "unknown"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cleaned.IsZero() || now.Sub(l.cleaned) >= l.ttl/2 {
		for candidate, visitor := range l.clients {
			if now.Sub(visitor.lastSeen) >= l.ttl {
				delete(l.clients, candidate)
			}
		}
		l.cleaned = now
	}
	visitor, exists := l.clients[key]
	if !exists {
		visitor = &clientLimiter{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.clients[key] = visitor
	}
	visitor.lastSeen = now
	return visitor.limiter
}

func RateLimit(limiter *ClientRateLimiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		allowed, retryAfter := limiter.Allow(ctx.ClientIP())
		if allowed {
			ctx.Next()
			return
		}
		seconds := int(math.Ceil(retryAfter.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		ctx.Header("Retry-After", strconv.Itoa(seconds))
		WriteError(ctx, http.StatusTooManyRequests, apperror.CodeTooManyRequests, "too many requests")
		ctx.Abort()
	}
}
