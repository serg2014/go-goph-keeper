package app

import (
	"context"
	"errors"

	"github.com/google/uuid"
	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/auth"
	"github.com/serg2014/go-goph-keeper/internal/client/config"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
)

const (
	maxRetries = 2
)

var (
	ErrEmptyRefreshToken = errors.New("empty refresh token")
)

type ClientApp struct {
	store       storage.Storager
	config      *config.Config
	grpcAuth    pb.AuthServiceClient
	grpcKeep    pb.GophKeeperServiceClient
	authManager *auth.AuthManager
}

func NewApp(store storage.Storager, config *config.Config) *ClientApp {
	return &ClientApp{
		store:       store,
		config:      config,
		authManager: auth.NewAuthManager(),
	}
}

func (app *ClientApp) SetAuthClient(authClient pb.AuthServiceClient) {
	app.grpcAuth = authClient
}

func (app *ClientApp) SetKeeperClient(keeperClient pb.GophKeeperServiceClient) {
	app.grpcKeep = keeperClient
}

func (app *ClientApp) AddSecret(ctx context.Context, secret *models.Secret) error {
	secretDB, err := app.NewSecretDBFromSecret(secret)
	if err != nil {
		return err
	}

	err = app.store.AddSecret(ctx, secretDB)
	if err != nil {
		return err
	}
	return nil
}

func (app *ClientApp) UpdateSecret(ctx context.Context, secret *models.Secret) error {
	secretDB, err := app.NewSecretDBFromSecret(secret)
	if err != nil {
		return err
	}
	err = app.store.UpdateSecret(ctx, secretDB)
	if err != nil {
		return err
	}
	return nil
}

func (app *ClientApp) SecretsList(ctx context.Context) ([]models.Secret, error) {
	list, err := app.store.SecretsList(ctx)
	if err != nil {
		return nil, err
	}

	secrets := make([]models.Secret, 0, len(list))
	for i := range list {
		item, err := app.NewSecretFromSecretDB(&list[i])
		if err != nil {
			return nil, err
		}
		// TODO
		item.Meta.Meta = nil
		secrets = append(secrets, *item)
	}
	return secrets, nil
}

func (app *ClientApp) GetSecret(ctx context.Context, id uuid.UUID) (*models.Secret, error) {
	secretDB, err := app.store.GetSecret(ctx, id)
	if err != nil {
		return nil, err
	}

	secret, err := app.NewSecretFromSecretDB(secretDB)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (app *ClientApp) DeleteSecret(ctx context.Context, id uuid.UUID, secretType models.SecretType) error {
	err := app.store.DeleteSecret(ctx, id)
	if err != nil {
		return err
	}
	if secretType == models.SecretTypeFile {
		return app.DeleteFileFromLocalStorage(id)
	}

	return nil
}

func (app *ClientApp) RegisterUser(ctx context.Context, login, password string) error {
	resp, err := app.grpcAuth.RegisterUser(ctx, &pb.RegisterUserRequest{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return err
	}

	app.authManager.SaveTokens(resp.Access.Token, resp.Refresh.Token)
	return nil
}

func (app *ClientApp) AuthUser(ctx context.Context, login, password string) error {
	resp, err := app.grpcAuth.AuthUser(ctx, &pb.AuthUserRequest{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return err
	}

	app.authManager.SaveTokens(resp.Access.Token, resp.Refresh.Token)
	return nil
}

func (app *ClientApp) RenewAuth(ctx context.Context) (context.Context, error) {
	// получаем refresh токен
	refresh := app.GetRefreshToken()
	if refresh == "" {
		return nil, ErrEmptyRefreshToken
	}
	// используем refresh токен для получения access токена
	ctx = app.authManager.AddAuthTokenToMeta(ctx, refresh)
	// пробуем обновить токен
	resp, err := app.grpcAuth.RenewAuth(ctx, &pb.RenewAuthRequest{})
	if err != nil {
		return nil, err
	}
	app.authManager.SaveTokens(resp.Access.Token, resp.Refresh.Token)
	// выставить в мета новый auth токен
	ctx = auth.AddAuthTokenToMeta(ctx, resp.Access.Token)
	return ctx, nil
}

func (app *ClientApp) Ping(ctx context.Context) error {
	_, err := app.grpcKeep.Ping(ctx, &pb.PingRequest{})
	return err
}

func (app *ClientApp) GetAuthToken() string {
	return app.authManager.GetAuthToken()
}

func (app *ClientApp) GetRefreshToken() string {
	return app.authManager.GetRefreshToken()
}
