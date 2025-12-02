package storage

import (
	"context"
	"errors"

	"github.com/google/uuid"
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

var (
	ErrMetaAndDataEmpty = errors.New("meta and data empty")
)

// Storager interface
type Storager interface {
	Close() error
	AddSecret(ctx context.Context, secret *models.SecretDB) error
	UpdateSecret(ctx context.Context, secret *models.SecretDB) error
	SecretsList(ctx context.Context) ([]models.SecretDB, error)
	GetSecret(ctx context.Context, secret_id uuid.UUID) (*models.SecretDB, error)
	DeleteSecret(ctx context.Context, secret_id uuid.UUID) error
	// sync
	GetSecretsIDsForCreate(ctx context.Context) ([]uuid.UUID, error)
	GetSecretForCreate(ctx context.Context, secret_id uuid.UUID) (*models.SecretDBCreateServer, error)
	UpdateSecretVersionAfterCreate(ctx context.Context, secret_id uuid.UUID, conflict bool) error

	GetSecretsIDsForServerUpdate(ctx context.Context) ([]uuid.UUID, error)
	GetSecretForUpdate(ctx context.Context, secret_id uuid.UUID) (*models.SecretDBUpdateServer, error)
	UpdateSecretVersionAfterUpdate(ctx context.Context, resp *pb.UpdateSecretResponse) error

	GetSecretsIDsForServerDelete(ctx context.Context) ([]uuid.UUID, error)
	GetSecretForDelete(ctx context.Context, secret_id uuid.UUID) (*models.SecretDBDeleteServer, error)
	UpdateSecretAfterDelete(ctx context.Context, resp *pb.DeleteSecretResponse) error
}
