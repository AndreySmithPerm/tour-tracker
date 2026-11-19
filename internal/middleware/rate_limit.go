package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

// Config holds rate limit thresholds
type Config struct {
	// MaxRequestsPerDayIP — максимальное число запросов в сутки с одного IP (без авторизации)
	MaxRequestsPerDayIP int
	// MaxRequestsPerDayUser — максимальное число запросов в сутки с одного авторизованного пользователя
	MaxRequestsPerDayUser int
	// MaxRegistrationsPerDayIP — максимум регистраций в сутки с одного IP
	MaxRegistrationsPerDayIP int
	// MaxSubscriptionsPerDayIP — максимум подписок в сутки с одного IP
	MaxSubscriptionsPerDayIP int
	// MaxLinkGenerationsPerDayIP — максимум генераций партнёрских ссылок в сутки с одного IP
	MaxLinkGenerationsPerDayIP int
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxRequestsPerDayIP:          500,   // 500 запросов в сутки с IP
		MaxRequestsPerDayUser:        2000,  // 2000 запросов в сутки с авторизованного пользователя
		MaxRegistrationsPerDayIP:     3,     // 3 регистрации в сутки с IP
		MaxSubscriptionsPerDayIP:     10,    // 10 подписок в сутки с IP
		MaxLinkGenerationsPerDayIP:   50,    // 50 генераций ссылок в сутки с IP
	}
}

// ContextKey for passing values through context
type ContextKey string

const (
	ClientIPKey ContextKey = "client_ip"
	UserIDKey   ContextKey = "user_id"
)

// ExtractIP extracts client IP from request (handles reverse proxy / CDN)
func ExtractIP(r *http.Request) string {
	// Check X-Forwarded-For first (behind reverse proxy / CDN)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP (set by Nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Strip port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		// Check if it's IPv6 (has multiple colons)
		if strings.Count(ip, ":") > 1 {
			// IPv6: [::1]:port
			if idx2 := strings.LastIndex(ip, "]"); idx2 != -1 {
				return ip[:idx2+1]
			}
		}
		ip = ip[:idx]
	}
	return ip
}

// ClientRequestMiddleware tracks all requests and enforces daily limits
func ClientRequestMiddleware(s *storage.Storage, cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := ExtractIP(r)

			// Store IP in context for downstream handlers
			ctx := context.WithValue(r.Context(), ClientIPKey, clientIP)
			r = r.WithContext(ctx)

			// Track request in DB (async, don't block)
			userID, _ := r.Context().Value(UserIDKey).(int64)
			go func() {
				_ = s.RecordClientRequest(context.Background(), clientIP, r.URL.Path, r.Method, &userID)
			}()

			// Skip limit check for health checks and static assets
			if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Check daily request limit for this IP
			count, err := s.CountRequestsTodayByIP(context.Background(), clientIP)
			if err != nil {
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			if count >= cfg.MaxRequestsPerDayIP {
				w.Header().Set("Retry-After", strconv.Itoa(86400-count%86400))
				http.Error(w, "Too many requests today. Please try again tomorrow.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RegistrationLimitMiddleware blocks registration if IP exceeded limit
func RegistrationLimitMiddleware(s *storage.Storage, cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := r.Context().Value(ClientIPKey).(string)

			count, err := s.CountRegistrationsTodayByIP(context.Background(), clientIP)
			if err != nil {
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			if count >= cfg.MaxRegistrationsPerDayIP {
				http.Error(w, "Registration limit exceeded. Max 3 registrations per day from one IP.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SubscriptionLimitMiddleware blocks subscription creation if IP exceeded limit
func SubscriptionLimitMiddleware(s *storage.Storage, cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := r.Context().Value(ClientIPKey).(string)

			count, err := s.CountActionsTodayByIP(context.Background(), clientIP, "subscribe")
			if err != nil {
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			if count >= cfg.MaxSubscriptionsPerDayIP {
				http.Error(w, "Subscription limit exceeded. Max 10 subscriptions per day from one IP.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LinkGenerationLimitMiddleware blocks affiliate link generation if IP exceeded limit
func LinkGenerationLimitMiddleware(s *storage.Storage, cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := r.Context().Value(ClientIPKey).(string)

			count, err := s.CountActionsTodayByIP(context.Background(), clientIP, "link_generate")
			if err != nil {
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			if count >= cfg.MaxLinkGenerationsPerDayIP {
				http.Error(w, "Link generation limit exceeded. Please try again tomorrow.", http.StatusTooManyRequests)
				return
			}

			_ = s.RecordRateLimit(context.Background(), clientIP, "link_generate")
			next.ServeHTTP(w, r)
		})
	}
}
