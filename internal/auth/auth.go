package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/serg2014/go-goph-keeper/internal/models"
)

const (
	tokenExpire        = 15 * time.Minute
	refreshTokenExpire = 1 * time.Hour
	// TODO from env
	secretForToken = "secretfortoken"
	// TODO from env
	secretForPassword = "somesecret"
)

var (
	ErrTokenExpired          = errors.New("jwt token is expired")
	ErrTokenBadSigningMethod = errors.New("unexpected signing method")

	// ErrUserIDFromContext error when no userid in context
	ErrUserIDFromContext = fmt.Errorf("no userid in context")
)

type userCtxKeyType string

const userCtxKey userCtxKeyType = "userID"

// WithUser helper set userid in context
func WithUser(ctx context.Context, userID *models.UserID) context.Context {
	return context.WithValue(ctx, userCtxKey, userID)
}

// GetUserID get userid from context
func GetUserIDFromContext(ctx context.Context) (*models.UserID, error) {
	userID, ok := ctx.Value(userCtxKey).(*models.UserID)
	if !ok {
		return nil, ErrUserIDFromContext
	}
	return userID, nil
}

func sign(value, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(value)
	return base64.RawStdEncoding.EncodeToString(h.Sum(nil))
}

func SignPassword(password string) string {
	return sign([]byte(password), []byte(secretForPassword))
}

// Claims — структура утверждений, которая включает стандартные утверждения
// и одно пользовательское — UserID
type Claims struct {
	jwt.RegisteredClaims
	UserID    *models.UserID
	IsRefresh bool
}

// BuildJWTString создаёт токен и возвращает его в виде строки.
func BuildJWTString(userID *models.UserID) (string, error) {
	return buildJWTString(userID, tokenExpire, false)
}

func BuildJWTRefreshString(userID *models.UserID) (string, error) {
	return buildJWTString(userID, refreshTokenExpire, true)
}

func buildJWTString(userID *models.UserID, expire time.Duration, refresh bool) (string, error) {
	// создаём новый токен с алгоритмом подписи HS256 и утверждениями — Claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			// когда создан токен
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
		},
		// собственное утверждение
		UserID:    userID,
		IsRefresh: refresh,
	})

	// создаём строку токена
	tokenString, err := token.SignedString([]byte(secretForToken))
	if err != nil {
		return "", err
	}

	// возвращаем строку токена
	return tokenString, nil
}

func GetUserIDFromToken(tokenString string) (*models.UserID, bool, error) {
	// создаём экземпляр структуры с утверждениями
	claims := &Claims{}
	// парсим из строки токена tokenString в структуру claims
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("%w: %v", ErrTokenBadSigningMethod, t.Header["alg"])
			}
			return []byte(secretForToken), nil
		})

	if token.Valid {
		// возвращаем ID пользователя в читаемом виде
		return claims.UserID, claims.IsRefresh, nil
	}

	if errors.Is(err, jwt.ErrTokenExpired) {
		return nil, claims.IsRefresh, ErrTokenExpired
	}

	return nil, claims.IsRefresh, err
}
