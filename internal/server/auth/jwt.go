package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/serg2014/go-goph-keeper/internal/server/models"
)

// Claims — структура утверждений, которая включает стандартные утверждения
// и одно пользовательское — UserID
type Claims struct {
	jwt.RegisteredClaims
	UserID    *models.UserID
	IsRefresh bool
}

// BuildAccessJWT создаёт access токен и возвращает его в виде строки.
func BuildAccessJWT(userID *models.UserID) (string, error) {
	return buildJWT(userID, accessTokenExpire, false)
}

// BuildAccessJWT создаёт refresh токен и возвращает его в виде строки.
func BuildRefreshJWT(userID *models.UserID) (string, error) {
	return buildJWT(userID, refreshTokenExpire, true)
}

func buildJWT(userID *models.UserID, expire time.Duration, refresh bool) (string, error) {
	// создаём новый токен с алгоритмом подписи HS256 и утверждениями — Claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			// когда токен перестанет быть валидным
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
