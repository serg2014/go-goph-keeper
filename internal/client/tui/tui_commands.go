package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/serg2014/go-goph-keeper/internal/client/app"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

var (
	Version   string
	BuildTime string
)

var (
	ErrSecretType = errors.New("unknown secret type")
)

func showSecretsList(ctx context.Context, app *app.ClientApp) error {
	sec := &models.Secret{}

	// TODO подумать о пагинации
	secrets, err := app.SecretsList(ctx)
	if err != nil {
		return err
	}

	opts := make([]huh.Option[*models.Secret], 0, len(secrets))
	for _, item := range secrets {
		b := strings.Builder{}
		b.WriteString(item.Type.String())
		b.WriteString(fmt.Sprintf(" %s ", item.ID.String()))
		b.WriteString(item.Meta.InternalMeta.SecretName)
		opts = append(opts, huh.NewOption(b.String(), &item))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[*models.Secret]().
				Key("secret").
				Title("List of secrets").
				Options(opts...).
				Value(&sec),
		).WithHideFunc(func() bool {
			return len(secrets) == 0
		}),
		huh.NewGroup(
			huh.NewNote().Description("No secrets"),
		).WithHideFunc(func() bool {
			return len(secrets) != 0
		}),
	)
	err = form.Run()
	if err != nil {
		return err
	}
	if sec.Type == models.SecretTypeUnknown {
		return nil
	}

	secret, err := app.GetSecret(ctx, sec.ID)
	if err != nil {
		return err
	}

	// show form for edit secret
	save := false
	isAddForm := false
	form, err = tuiFormAddOrEditSecret(secret, &save, isAddForm, app.DescryptFileFromLocalStorage)
	if err != nil {
		return err
	}

	err = form.Run()
	if err != nil {
		return err
	}

	if secret.Type == models.SecretTypeFile && secret.Data.FilePath.TmpPath != "" {
		os.Remove(secret.Data.FilePath.TmpPath)
	}
	if save {
		err = app.UpdateSecret(ctx, secret)
		if err != nil {
			return err
		}
	} else if form.GetBool("delete") {
		err = app.DeleteSecret(ctx, secret.ID, secret.Type)
		if err != nil {
			return err
		}
	}
	return nil
}

func addSecret(ctx context.Context, app *app.ClientApp) error {
	var secretType models.SecretType
	opts := make([]huh.Option[models.SecretType], 0, models.SecretTypeMax)
	for i := 0; i < int(models.SecretTypeMax); i++ {
		str := models.SecretType(i).String()
		if str != "" {
			opts = append(opts, huh.NewOption(str, models.SecretType(i)))
		}
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[models.SecretType]().
				Title("Type of secret").
				Options(opts...).
				Value(&secretType),
		),
	)
	err := form.Run()
	if err != nil {
		return err
	}

	secret := &models.Secret{
		ID:   uuid.New(),
		Type: secretType,
		Meta: models.BlockMeta{
			Meta: make(models.Meta),
		},
	}

	save := false
	// show form for add secret
	isAddForm := true
	form, err = tuiFormAddOrEditSecret(secret, &save, isAddForm, app.DescryptFileFromLocalStorage)
	if err != nil {
		return err
	}

	err = form.Run()
	if err != nil {
		return err
	}

	if save {
		err = app.AddSecret(ctx, secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func showSyncMenu(ctx context.Context, app *app.ClientApp) error {
	opts := make([]huh.Option[SyncMenuType], 0, SyncMenuTypeMax)
	for i := SyncMenuType(0); i < SyncMenuTypeMax; i++ {
		str := i.String()
		if str != "" {
			opts = append(opts, huh.NewOption(str, i))
		}
	}
	var selectedMenu SyncMenuType
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[SyncMenuType]().
				Title("Sync").
				Options(opts...).
				Value(&selectedMenu),
		),
	)
	err := form.Run()
	if err != nil {
		return err
	}

	err = tuiSyncForm(ctx, app, selectedMenu)
	if err != nil {
		return err
	}

	return nil
}

func showVersion(ctx context.Context, app *app.ClientApp) error {
	desc := fmt.Sprintf("Version: %s\nBuild time: %s", Version, BuildTime)
	logger.Logger.Debug(desc)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Version").
				DescriptionFunc(func() string {
					return desc
				}, &desc),
		),
	)
	return form.Run()
}
