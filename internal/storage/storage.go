package storage

import (
	"context"
	"errors"

	"github.com/serg2014/go-goph-keeper/internal/models"
)

var ErrUserExists = errors.New("user exists")

// Storager interface
type Storager interface {
	CreateUser(ctx context.Context, login, passwordHash string) (*models.UserID, error)
}
