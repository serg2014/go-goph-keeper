package app

import (
	"context"
	"encoding/json"

	"github.com/serg2014/go-goph-keeper/internal/client/models"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
)

type ClientApp struct {
	store storage.Storager
}

func NewApp(store storage.Storager) *ClientApp {
	return &ClientApp{store: store}
}

func (app *ClientApp) AddSecret(ctx context.Context, secret *models.Secret) error {
	data := &models.SecretDB{
		ID:   secret.ID,
		Type: secret.Type,
	}
	// TODO need crypt
	bm, err := json.Marshal(secret.Meta)
	if err != nil {
		return err
	}
	data.Meta = bm

	// TODO need crypt
	bd, err := json.Marshal(secret.Data)
	if err != nil {
		return err
	}
	data.Data = bd

	err = app.store.AddSecret(ctx, data)
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
		item := models.Secret{
			ID:   list[i].ID,
			Type: list[i].Type,
		}
		// TODO decrypt
		err := json.Unmarshal(list[i].Meta, &item.Meta)
		if err != nil {
			return nil, err
		}
		item.Name = item.Meta[models.MetaKeyName]
		item.Meta = nil
		secrets = append(secrets, item)
	}
	return secrets, nil
}
