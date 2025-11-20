package app

import (
	"context"

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
