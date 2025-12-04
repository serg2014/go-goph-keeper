package app

import (
	"context"
	"errors"
	"sync"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/server/auth"
	"github.com/serg2014/go-goph-keeper/internal/server/config"
	"github.com/serg2014/go-goph-keeper/internal/server/models"
	"github.com/serg2014/go-goph-keeper/internal/server/storage"
)

type MyApp struct {
	store         storage.Storager
	config        *config.Config
	activeUploads map[string]struct{}
	mapMutex      sync.Mutex
}

func NewApp(store storage.Storager, conf *config.Config) *MyApp {
	return &MyApp{
		store:         store,
		config:        conf,
		activeUploads: make(map[string]struct{}),
		mapMutex:      sync.Mutex{},
	}
}

func (app *MyApp) Ping(ctx context.Context) error {
	return nil
}

func (app *MyApp) CreateUser(ctx context.Context, login, password string) (*models.UserID, error) {
	return app.store.CreateUser(ctx, login, auth.SignPassword(password))
}

func (app *MyApp) GetUser(ctx context.Context, login, password string) (*models.UserID, error) {
	return app.store.GetUser(ctx, login, auth.SignPassword(password))
}

func (app *MyApp) CreateSecret(ctx context.Context, req *pb.CreateSecretRequest) (*pb.CreateSecretResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return app.store.CreateSecret(ctx, *userID, req)
}

func (app *MyApp) UpdateSecret(ctx context.Context, req *pb.UpdateSecretRequest) (*pb.UpdateSecretResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return app.store.UpdateSecret(ctx, *userID, req)
}

func (app *MyApp) DeleteSecret(ctx context.Context, req *pb.DeleteSecretRequest) (*pb.DeleteSecretResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.IsFile {
		// если файл уже загружается выходим
		if !app.getUploadFlag(req.Id) {
			return nil, ErrFileUploading
		}
		defer app.cleanUploadFlag(req.Id)
	}

	res, err := app.store.DeleteSecret(ctx, *userID, req)
	if err != nil && !errors.Is(err, storage.ErrConflict) {
		return nil, err
	}

	if req.IsFile && !errors.Is(err, storage.ErrConflict) {
		err := app.deleteSecretFile(userID, req.Id)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (app *MyApp) GetSecretsListInfo(ctx context.Context) ([]*pb.SecretsListResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return app.store.GetSecretsListInfo(ctx, *userID)
}

func (app *MyApp) GetSecret(ctx context.Context, req *pb.GetSecretsRequest) (*pb.GetSecretsResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return app.store.GetSecret(ctx, *userID, req.Id)
}
