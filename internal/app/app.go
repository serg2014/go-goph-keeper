package app

import (
	"context"

	"github.com/serg2014/go-goph-keeper/internal/auth"
	"github.com/serg2014/go-goph-keeper/internal/models"
	"github.com/serg2014/go-goph-keeper/internal/storage"
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
