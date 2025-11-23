package storage

import (
	"context"

	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

// Storager interface
type Storager interface {
	Close() error
	AddSecret(ctx context.Context, secret *models.SecretDB) error
	SecretsList(ctx context.Context) ([]models.SecretDB, error)
}
