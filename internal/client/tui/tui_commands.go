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

	// show form for edit secret
	//secret := &models.Secret{ID: sec.ID, Type: sec.Type, Name: sec.Name, Meta: make(models.Meta)}
	secret, err := app.GetSecret(ctx, sec.ID)
	if err != nil {
		return err
	}

	form, err = tuiFormAddOrEditSecret(secret)
	if err != nil {
		return err
	}

	// switch secret.Type {
	// case models.SecretTypeCreditCard:
	// 	// TODO
	// 	secret.Data.CreditCard = &models.CreditCard{}
	// 	form = tuiFormCredirCard(secret)
	// case models.SecretTypeLogingPassword:
	// 	// TODO
	// 	secret.Data.LoginPassword = &models.LoginPassword{}
	// 	form = tuiFormLoginPassword(secret)
	// default:
	// 	return ErrSecretType
	// }

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

	form, err = tuiFormAddOrEditSecret(secret)
	if err != nil {
		return err
	}

	err = form.Run()
	if err != nil {
		return err
	}

	if form.GetBool("save") {
		err = app.AddSecret(ctx, secret)
		if err != nil {
			return err
		}
	}
	return nil
}
