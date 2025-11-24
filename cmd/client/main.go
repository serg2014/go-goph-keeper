package main

import (
	"context"

	"github.com/serg2014/go-goph-keeper/internal/client/app"
	"github.com/serg2014/go-goph-keeper/internal/client/config"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
	"github.com/serg2014/go-goph-keeper/internal/client/tui"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}
	defer conf.Clean()

	ctx := context.Background()
	store, err := storage.NewStorageDB(ctx, conf.DbPath())
	if err != nil {
		return err
	}

	app := app.NewApp(store, conf)

	err = tui.Tui(ctx, app)
	if err != nil {
		return err
	}

	return nil
}
