package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
)

type JWTConfig struct {
	SecretKey string
	Expire    time.Duration
}

func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SecretKey: "change_me_to_secure_random_string",
		Expire:    24 * time.Hour,
	}
}

type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(cfg JWTConfig, userID int64, email, username string) (string, error) {
	claims := JWTClaims{
		UserID: userID, Email: email, Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.Expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.SecretKey))
}

func VerifyToken(cfg JWTConfig, tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.SecretKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

const (
	EmailKey   ContextKey = "email"
	UsernameKey ContextKey = "username"
	IsAuthKey  ContextKey = "is_authenticated"
)

func JWTAuthMiddleware(cfg JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"Authorization header required"}`, http.StatusUnauthorized)
				return
			}
			parts := splitOnce(authHeader, ' ')
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error":"Invalid authorization format"}`, http.StatusUnauthorized)
				return
			}
			claims, err := VerifyToken(cfg, parts[1])
			if err != nil {
				http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)
			ctx = context.WithValue(ctx, UsernameKey, claims.Username)
			ctx = context.WithValue(ctx, IsAuthKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthRequiredMiddleware(cfg JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Требуется авторизация"})
				return
			}
			parts := splitOnce(authHeader, ' ')
			if len(parts) != 2 || parts[0] != "Bearer" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Неверный формат токена"})
				return
			}
			claims, err := VerifyToken(cfg, parts[1])
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Токен недействителен"})
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)
			ctx = context.WithValue(ctx, UsernameKey, claims.Username)
			ctx = context.WithValue(ctx, IsAuthKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type MaxWebhookPayload struct {
	Event     string `json:"event"`
	UserID    string `json:"user_id"`
	ChatID    string `json:"chat_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signed_at"`
}

func VerifyMAXWebhook(payload MaxWebhookPayload, secret string) bool {
	signData := fmt.Sprintf("%s|%s|%d", payload.UserID, payload.ChatID, payload.SignedAt)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signData))
	expectedSig := hex.EncodeToString(h.Sum(nil))
	return payload.Signature == expectedSig
}

func MaxWebhookHandler(s *storage.Storage, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		var payload MaxWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
			return
		}
		if secret != "" && !VerifyMAXWebhook(payload, secret) {
			log.Printf("[MAX] Invalid signature from user %s", payload.UserID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid signature"})
			return
		}
		log.Printf("[MAX] Event: %s user=%s", payload.Event, payload.UserID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func splitOnce(s string, c rune) []string {
	for i := 0; i < len(s); i++ {
		if rune(s[i]) == c {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}