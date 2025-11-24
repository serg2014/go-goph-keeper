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
	metaStr, err := meta.PrettyString()
	if err != nil {
		// TODO залогировать ошибку
	}
	return huh.NewText().
		Title("Meta info").
		Description("json dict format. Key __name__ is not allowed to be used.").
		Validate(func(data string) error {
			clear(meta)
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
		Value(&metaStr).
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

func tuiFormCredirCard(secret *models.Secret, save *bool) *huh.Form {
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
				}).
				Value(&secret.Data.CreditCard.Number),
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
				}).
				Value(&secret.Data.CreditCard.Exp),
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
				}).
				Value(&secret.Data.CreditCard.Cvv),
			tuiMeta(secret.Meta),
			tuiComfirm(secret, save),
		),
		tuiConfirmDelete(secret, save),
	)
}

func tuiFormLoginPassword(secret *models.Secret, save *bool) *huh.Form {
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
			tuiComfirm(secret, save),
		),
		tuiConfirmDelete(secret, save),
	)
}

func tuiComfirm(secret *models.Secret, save *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title("Save changes?").
		Key("save").
		Value(save)
}

func tuiFormAddOrEditSecret(secret *models.Secret, save *bool, fn func(*models.Secret) (string, error)) (*huh.Form, error) {
	var form *huh.Form
	switch secret.Type {
	case models.SecretTypeLogingPassword:
		if secret.Data.LoginPassword == nil {
			secret.Data.LoginPassword = &models.LoginPassword{}
		}
		form = tuiFormLoginPassword(secret, save)
	case models.SecretTypeCreditCard:
		if secret.Data.CreditCard == nil {
			secret.Data.CreditCard = &models.CreditCard{}
		}
		form = tuiFormCredirCard(secret, save)
	case models.SecretTypeText:
		form = tuiFormText(secret, save)
	case models.SecretTypeFile:
		form = tuiFormFile(secret, save, fn)
	default:
		return nil, ErrSecretType
	}

	return form, nil
}

func tuiFormText(secret *models.Secret, save *bool) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			secretNameInput(&secret.Name),
			huh.NewText().
				Key("Text").
				Title("Text").
				Validate(func(data string) error {
					if data == "" {
						return ErrRequiredField
					}
					return nil
				}).
				Value(&secret.Data.Text),
			tuiMeta(secret.Meta),
			tuiComfirm(secret, save),
		),
		tuiConfirmDelete(secret, save),
	)
}

func tuiFormFile(secret *models.Secret, save *bool, fn func(*models.Secret) (string, error)) *huh.Form {
	secret.Data.File.Path = ""
	opts := []huh.Field{
		secretNameInput(&secret.Name),
		huh.NewFilePicker().
			ShowHidden(true). // do not work
			ShowSize(true).
			Key("File").
			Title("File").
			Value(&secret.Data.File.Path),
	}
	if len(secret.Data.File.Data) != 0 {
		desc := fmt.Sprintf("File with size: %d in vault", len(secret.Data.File.Data))
		opts = append(opts,
			huh.NewConfirm().
				DescriptionFunc(func() string {
					return desc
				}, &desc).
				Title("Show file?").
				Key("download_file").
				Validate(func(b bool) error {
					if b {
						path, err := fn(secret)
						if err != nil {
							return err
						}
						desc += fmt.Sprintf("\nFile in %s\n", path)
					}
					return nil
				}),
		)
	}

	opts = append(opts,
		tuiMeta(secret.Meta),
		tuiComfirm(secret, save).
			Validate(func(b bool) error {
				if b {
					if secret.ID == 0 && secret.Data.File.Path == "" {
						return fmt.Errorf("file: %w", ErrRequiredField)
					}
				}
				return nil
			}),
	)
	return huh.NewForm(
		huh.NewGroup(opts...),
		tuiConfirmDelete(secret, save),
	)
}

func tuiConfirmDelete(secret *models.Secret, save *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Delete secret?").
			Key("delete")).
		WithHideFunc(func() bool {
			return secret.ID == 0 || *save
		})
}
