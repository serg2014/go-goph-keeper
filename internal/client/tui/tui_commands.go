package tui

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/serg2014/go-goph-keeper/internal/client/app"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

var (
	ErrSecretType = errors.New("unknown secret type")
)

// TODO когда нет секретов нет возможности выйти из меню.
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
		b.WriteString(item.Name)
		b.WriteString(" ")
		b.WriteString(item.Type.String())
		opts = append(opts, huh.NewOption(b.String(), &item))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[*models.Secret]().
				Key("secret").
				Title("List of secrets").
				Options(opts...).
				Value(&sec),
		),
	)
	err = form.Run()
	if err != nil {
		return err
	}

	secret, err := app.GetSecret(ctx, sec.ID)
	if err != nil {
		return err
	}

	// show form for edit secret
	save := false
	form, err = tuiFormAddOrEditSecret(secret, &save, app.SaveSecterToFile)
	if err != nil {
		return err
	}

	err = form.Run()
	if err != nil {
		return err
	}

	if save {
		err = app.UpdateSecret(ctx, secret)
		if err != nil {
			return err
		}
	} else if form.GetBool("delete") {
		err = app.DeleteSecret(ctx, secret.ID)
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
		Type: secretType,
		Meta: make(models.Meta),
	}

	save := false
	form, err = tuiFormAddOrEditSecret(secret, &save, app.SaveSecterToFile)
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
