package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/serg2014/go-goph-keeper/internal/server/models"
)

const (
	accessTokenExpire  = 1 * time.Minute
	refreshTokenExpire = 1 * time.Hour
	// TODO from env
	secretForToken = "secretfortoken"
	// TODO from env
	secretForPassword = "somesecret"
)

var (
	ErrTokenExpired          = errors.New("jwt token is expired")
	ErrTokenBadSigningMethod = errors.New("unexpected signing method")
	ErrTokenRequired         = errors.New("auth token required")

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
