package app

import (
	"context"
	"os"
	"path"
	"strconv"

	"github.com/serg2014/go-goph-keeper/internal/client/config"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
)

type ClientApp struct {
	store  storage.Storager
	config *config.Config
}

func NewApp(store storage.Storager, config *config.Config) *ClientApp {
	return &ClientApp{store: store, config: config}
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

func (app *ClientApp) SaveSecterToFile(secret *models.Secret) (string, error) {
	path := path.Join(app.config.TmpDirPath(), strconv.Itoa(secret.ID))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}

	off := 0
	max := len(secret.Data.File.Data)
	for i := 0; i < max; i += MaxChunkSizeBytes {
		j := i + MaxChunkSizeBytes
		if j > max {
			j = max
		}
		n, err := f.WriteAt(secret.Data.File.Data[i:j], int64(off))
		if err != nil {
			return "", err
		}
		off += n
	}
	return path, nil
}
