package tui

import (
	"context"
	"errors"

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

	secrets, err := app.SecretsList(ctx)
	if err != nil {
		return err
	}
	// secrets := []models.Secret{
	// 	models.Secret{ID: -1, Name: "first secret", Type: models.SecretTypeLogingPassword},
	// 	models.Secret{ID: 2, Name: "second secret", Type: models.SecretTypeCreditCard},
	// }
	opts := make([]huh.Option[*models.Secret], 0, len(secrets))
	for _, item := range secrets {
		opts = append(opts, huh.NewOption(item.Name, &item))
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

	secret := &models.Secret{ID: sec.ID, Type: sec.Type, Name: sec.Name, Meta: make(models.Meta)}

	switch secret.Type {
	case models.SecretTypeCreditCard:
		// TODO
		secret.Data.CreditCard = &models.CreditCard{}
		form = tuiFormCredirCard(secret)
	case models.SecretTypeLogingPassword:
		// TODO
		secret.Data.LoginPassword = &models.LoginPassword{}
		form = tuiFormLoginPassword(secret)
	default:
		return ErrSecretType
	}

	err = form.Run()
	if err != nil {
		return err
	}
	return nil

}

func addSecret(ctx context.Context, app *app.ClientApp) error {
	var secretType models.SecretType
	opts := make([]huh.Option[models.SecretType], 0, models.SecretTypeMax)
	for i := 0; i < int(CommandTypeMax); i++ {
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
	switch secretType {
	case models.SecretTypeLogingPassword:
		secret.Data.LoginPassword = &models.LoginPassword{}
		form = tuiFormLoginPassword(secret)
	case models.SecretTypeCreditCard:
		secret.Data.CreditCard = &models.CreditCard{}
		form = tuiFormCredirCard(secret)
	default:
		return ErrSecretType
	}

	err = form.Run()
	if err != nil {
		return err
	}

	err = app.AddSecret(ctx, secret)
	if err != nil {
		return err
	}
	return nil
}
