package app

import (
	"context"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/server/auth"
	"github.com/serg2014/go-goph-keeper/internal/server/models"
	"github.com/serg2014/go-goph-keeper/internal/server/storage"
)

type MyApp struct {
	store storage.Storager
}

func NewApp(store storage.Storager) *MyApp {
	return &MyApp{
		store: store,
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
	return app.store.DeleteSecret(ctx, *userID, req)
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
