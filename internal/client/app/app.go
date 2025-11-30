package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/auth"
	"github.com/serg2014/go-goph-keeper/internal/client/config"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
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

func (app *ClientApp) Sync(ctx context.Context) error {
	/*
		1. Удаляем секреты на сервере по записям из таблицы deleted
		2. Создаем секреты на сервере (все записи с отрицательными ключами)
		3. Обновляем секреты на сервере
		4. Обновляем секреты локально
		5. Удаляем секреты локально
		6. Создаем секреты локально
	*/
	return app.syncCreateSecret(ctx)
	//return errors.New("not implemented")
}

func (app *ClientApp) syncCreateSecret(ctx context.Context) error {
	// CreateSecrets(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[CreateSecretsRequest, CreateSecretsResponse], error)
	stream, err := app.grpcKeep.CreateSecrets(ctx)
	if err != nil {
		return fmt.Errorf("grpc CreateSecrets: %v", err)
	}

	waitResponse := make(chan error)
	// go routine to receive responses
	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				logger.Logger.Debug("no more responses")
				waitResponse <- nil
				return
			}
			if err != nil {
				waitResponse <- fmt.Errorf("cannot receive stream response: %v", err)
				return
			}

			logger.Logger.Debug("received response", slog.Uint64("server_id", res.Secret.ServerId))
		}
	}()

	// 	message SecretData {
	//     int64 id = 1;
	//     int64 version = 2;
	//     int64 updated_at = 3;
	//     bytes data = 4;
	// }
	// message Secret {
	//     int64 id = 1;
	//     SecretData meta = 2;
	//     SecretData data = 3;

	// send requests
	for i := range 11 {
		if i == 0 {
			continue
		}
		req := &pb.CreateSecretRequest{
			Secret: &pb.Secret{
				Id: int64(-1 * i),
				Meta: &pb.SecretData{
					Id:        int64(-1 * i),
					UpdatedAt: 100,
					Data:      []byte("ssss"),
				},
			},
		}

		err := stream.Send(req)
		if err != nil {
			return fmt.Errorf("cannot send stream request: %v - %v", err, stream.RecvMsg(nil))
		}

		logger.Logger.Debug("sent request", slog.String("req", fmt.Sprintf("%v", req)))

	}

	err = stream.CloseSend()
	if err != nil {
		return fmt.Errorf("cannot close send: %v", err)
	}

	err = <-waitResponse
	return err
}
