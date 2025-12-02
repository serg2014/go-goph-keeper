package storage

import (
	"context"

	"github.com/google/uuid"
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

// Storager interface
type Storager interface {
	Close() error
	AddSecret(ctx context.Context, secret *models.SecretDB) error
	UpdateSecret(ctx context.Context, secret *models.SecretDB) error
	SecretsList(ctx context.Context) ([]models.SecretDB, error)
	GetSecret(ctx context.Context, id uuid.UUID) (*models.SecretDB, error)
	DeleteSecret(ctx context.Context, id uuid.UUID) error
	// sync
	GetSecretsIDsForCreate(ctx context.Context) ([]int64, error)
	GetSecretForCreate(ctx context.Context, id int64) (*models.SecretDBCreateServer, error)
	MoveSecret(ctx context.Context, data *pb.CreateSecretResponse) error
	GetSecretsIDsForServerUpdate(ctx context.Context) ([]int64, error)
	GetSecretForUpdate(ctx context.Context, id int64) (*models.SecretDBUpdateServer, error)
	UpdateSecretVersion(ctx context.Context, data *pb.UpdateSecretResponse) error
}
