package storage

import (
	"context"
	"errors"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/server/models"
)

var (
	ErrUserExists     = errors.New("user exists")
	ErrUserOrPassword = errors.New("bad user or password")
	ErrSecretExists   = errors.New("secret exists")
	ErrMetaExists     = errors.New("meta exists")
	ErrDataExists     = errors.New("data exists")
)

// Storager interface
type Storager interface {
	Close() error
	CreateUser(ctx context.Context, login, passwordHash string) (*models.UserID, error)
	GetUser(ctx context.Context, login, passwordHash string) (*models.UserID, error)
	CreateSecret(ctx context.Context, userID models.UserID, req *pb.CreateSecretRequest) (*pb.CreateSecretResponse, error)
	UpdateSecret(ctx context.Context, userID models.UserID, req *pb.UpdateSecretRequest) (*pb.UpdateSecretResponse, error)
}
