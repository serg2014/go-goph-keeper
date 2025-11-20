package storage

import (
	"context"
	"errors"

	"github.com/serg2014/go-goph-keeper/internal/server/models"
)

var (
	ErrUserExists     = errors.New("user exists")
	ErrUserOrPassword = errors.New("bad user or password")
)

// Storager interface
type Storager interface {
	CreateUser(ctx context.Context, login, passwordHash string) (*models.UserID, error)
	GetUser(ctx context.Context, login, passwordHash string) (*models.UserID, error)
}
