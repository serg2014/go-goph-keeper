package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc/metadata"
)

const (
	authHeader = "authorization"
	authType   = "bearer"
)

var (
	ErrNeedRetry = errors.New("need retry")
)

type AuthManager struct {
	mu      sync.RWMutex
	auth    string
	refresh string
}

func NewAuthManager() *AuthManager {
	return &AuthManager{}
}

func (a *AuthManager) SaveTokens(auth, refresh string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.auth = auth
	a.refresh = refresh
}

func (a *AuthManager) GetAuthToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.auth
}

func (a *AuthManager) GetRefreshToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.refresh
}

func (a *AuthManager) AddAuthTokenToMeta(ctx context.Context, token string) context.Context {
	return AddAuthTokenToMeta(ctx, token)
}

func AddAuthTokenToMeta(ctx context.Context, token string) context.Context {
	if token != "" {
		v := fmt.Sprintf("%s %s", authType, token)
		md, ok := metadata.FromOutgoingContext(ctx)
		if ok {
			md.Set(authHeader, v)
		} else {
			md = metadata.New(map[string]string{authHeader: v})
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}
