package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

var (
	ErrNotDict       = errors.New("not dict[string]string")
	ErrRequiredField = errors.New("required field")
	ErrExpireFormat  = errors.New("need format MM/YY")
	ErrOnlyDigits    = errors.New("only digits")
	ErrCardCvvLength = errors.New("3 digit")
	ErrCardLength    = errors.New("need 16 digits")
	ErrCardMonth     = errors.New("month between 1 and 12")
	ErrMetaName      = fmt.Errorf("bad key %s", models.MetaKeyName)
)

func tuiMeta(meta models.Meta) *huh.Text {
	return huh.NewText().
		Title("Meta info").
		Description("json dict format. Key __name__ is not allowed to be used.").
		Validate(func(data string) error {
			if data == "" {
				return nil
			}
			err := json.Unmarshal([]byte(data), &meta)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrNotDict, err)
			}
			if _, ok := meta[models.MetaKeyName]; ok {
				return ErrMetaName
			}
			return nil
		}).
		Key("meta")
}

func secretNameInput(name *string) *huh.Input {
	return huh.NewInput().
		Title("Secret name").
		Key("name").
		Validate(func(data string) error {
			if data == "" {
				return ErrRequiredField
			}
			return nil
		}).
		Value(name)
}

func tuiFormCredirCard(secret *models.Secret) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			secretNameInput(&secret.Name),
			huh.NewInput().
				Title("Card number").
				Key("card_number").
				CharLimit(16).
				Validate(func(data string) error {
					if data == "" {
						return ErrRequiredField
					}
					if len(data) != 16 {
						return ErrCardLength
					}
					_, err := strconv.ParseInt(data, 10, 64)
					if err != nil {
						return ErrOnlyDigits
					}
					secret.Data.CreditCard.Number = data
					return nil
				}),
			huh.NewInput().
				Title("Exp").
				Description("MM/YY").
				Key("card_expire").
				CharLimit(5).
				Validate(func(data string) error {
					if data == "" {
						return ErrRequiredField
					}

					if len(data) != 5 {
						return ErrExpireFormat
					}
					parts := strings.Split(data, "/")
					if len(parts) != 2 {
						return ErrExpireFormat
					}
					i, err := strconv.ParseInt(parts[0], 10, 32)
					if err != nil {
						return ErrOnlyDigits
					}
					if i < 1 || i > 12 {
						return ErrCardMonth
					}
					_, err = strconv.ParseInt(parts[1], 10, 32)
					if err != nil {
						return ErrOnlyDigits
					}

					secret.Data.CreditCard.Exp = data
					return nil
				}),
			huh.NewInput().
				Key("card_cvv").
				Title("cvv").
				CharLimit(3).
				Validate(func(data string) error {
					if data == "" {
						return ErrRequiredField
					}
					if len(data) != 3 {
						return ErrCardCvvLength
					}
					_, err := strconv.ParseInt(data, 10, 32)
					if err != nil {
						return ErrOnlyDigits
					}
					secret.Data.CreditCard.Cvv = data
					return nil
				}),
			tuiMeta(secret.Meta),
			tuiComfirm(secret),
		),
	)
}

func tuiFormLoginPassword(secret *models.Secret) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			secretNameInput(&secret.Name),
			huh.NewInput().
				Key("login").
				Title("Login").
				Validate(func(data string) error {
					if data == "" {
						return ErrRequiredField
					}
					return nil
				}).
				Value(&secret.Data.LoginPassword.Login),
			huh.NewInput().
				Key("password").
				Title("Password").
				Validate(func(data string) error {
					if data == "" {
						return ErrRequiredField
					}
					return nil
				}).
				Value(&secret.Data.LoginPassword.Password),
			tuiMeta(secret.Meta),
			tuiComfirm(secret),
		),
	)
}

func tuiComfirm(secret *models.Secret) *huh.Confirm {
	return huh.NewConfirm().
		Title("Save changes?").
		Key("save").
		Validate(func(b bool) error {
			if b {
				secret.Meta[models.MetaKeyName] = secret.Name
			}
			return nil
		})
}
