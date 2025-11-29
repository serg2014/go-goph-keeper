package app

import (
	"context"
	"errors"
	"os"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/config"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
)

type ClientApp struct {
	store    storage.Storager
	config   *config.Config
	grpcAuth pb.AuthServiceClient
	grpcKeep pb.GophKeeperServiceClient
	tokens   tokens
}

type tokens struct {
	auth    string
	refresh string
}

func NewApp(store storage.Storager, config *config.Config) *ClientApp {
	return &ClientApp{store: store, config: config}
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
		item.Meta = nil
		secrets = append(secrets, *item)
	}
	return secrets, nil
}

func (app *ClientApp) GetSecret(ctx context.Context, id int) (*models.Secret, error) {
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

func (app *ClientApp) DeleteSecret(ctx context.Context, id int, filePath string) error {
	err := app.store.DeleteSecret(ctx, id)
	if err != nil {
		return err
	}
	if filePath != "" {
		return os.Remove(filePath)
	}

	return nil
}

func (app *ClientApp) saveTokens(auth, refresh string) {
	app.tokens = tokens{
		auth:    auth,
		refresh: refresh,
	}
}
func (app *ClientApp) RegisterUser(ctx context.Context, login, password string) error {
	resp, err := app.grpcAuth.RegisterUser(ctx, &pb.RegisterUserRequest{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return err
	}

	app.saveTokens(resp.Access.Token, resp.Refresh.Token)
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

	app.saveTokens(resp.Access.Token, resp.Refresh.Token)
	return nil
}

func (app *ClientApp) RenewAuth(ctx context.Context) error {
	resp, err := app.grpcAuth.RenewAuth(ctx, &pb.RenewAuthRequest{})
	if err != nil {
		return err
	}
	app.saveTokens(resp.Access.Token, resp.Refresh.Token)
	return nil
}

func (app *ClientApp) Ping(ctx context.Context) error {
	_, err := app.grpcKeep.Ping(ctx, &pb.PingRequest{})
	return err
}

func (app *ClientApp) GetAuthToken() string {
	return app.tokens.auth
}

func (app *ClientApp) GetRefreshToken() string {
	return app.tokens.refresh
}

type SyncStatus struct {
	Remote     Status
	Local      Status
	Conflicted []Conflicted
}

type Status struct {
	Added   int
	Updated int
	Deleted int
}

type Conflicted struct {
	Meta ConflictedID
	Data ConflictedID
}

type ConflictedID struct {
	LocalID  int
	RemoteID int
}

func (app *ClientApp) Sync() (*SyncStatus, error) {
	/*
		1. Удаляем секреты на сервере по записям из таблицы deleted
		2. Создаем секреты на сервере (все записи с отрицательными ключами)
		3. Обновляем секреты на сервере
		4. Обновляем секреты локально
		5. Удаляем секреты локально
		6. Создаем секреты локально
	*/
	return nil, errors.New("not implemented")
}
