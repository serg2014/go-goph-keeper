package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/models"
)

const (
	MaxMetaSize = 1 * 1024 * 1024  // 1M
	MaxTextSize = 1 * 1024 * 1024  // 1M
	MaxFileSize = 50 * 1024 * 1024 // 50M
)

var (
	ErrNotDict       = errors.New("not dict[string]string")
	ErrRequiredField = errors.New("required field")
	ErrExpireFormat  = errors.New("need format MM/YY")
	ErrOnlyDigits    = errors.New("only digits")
	ErrCardCvvLength = errors.New("3 digit")
	ErrCardLength    = errors.New("need 16 digits")
	ErrCardMonth     = errors.New("month between 1 and 12")
	ErrMaxFileSize   = fmt.Errorf("max file size %dM", MaxFileSize/1024/1024)
)

func tuiFormAddOrEditSecret(secret *models.Secret, save *bool, isAddForm bool, fn func(string, string) (string, error)) (*huh.Form, error) {
	var form *huh.Form
	switch secret.Type {
	case models.SecretTypeLogingPassword:
		if secret.Data.LoginPassword == nil {
			secret.Data.LoginPassword = &models.LoginPassword{}
		}
		form = tuiFormLoginPassword(secret, save, isAddForm)
	case models.SecretTypeCreditCard:
		if secret.Data.CreditCard == nil {
			secret.Data.CreditCard = &models.CreditCard{}
		}
		form = tuiFormCredirCard(secret, save, isAddForm)
	case models.SecretTypeText:
		form = tuiFormText(secret, save, isAddForm)
	case models.SecretTypeFile:
		if secret.Data.FilePath == nil {
			secret.Data.FilePath = &models.FilePath{}
		}
		form = tuiFormFile(secret, save, isAddForm, fn)
	default:
		return nil, ErrSecretType
	}

	return form, nil
}

func tuiFormLoginPassword(secret *models.Secret, save *bool, isAddForm bool) *huh.Form {
	opts := []huh.Field{
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
	}
	return tuiFormHelper(secret, save, isAddForm, opts)
}

func tuiFormCredirCard(secret *models.Secret, save *bool, isAddForm bool) *huh.Form {
	opts := []huh.Field{
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
	}
	return tuiFormHelper(secret, save, isAddForm, opts)
}

func tuiFormText(secret *models.Secret, save *bool, isAddForm bool) *huh.Form {
	opts := []huh.Field{
		huh.NewText().
			Key("Text").
			Title("Text").
			Description(fmt.Sprintf("Max charecter %d", MaxTextSize)).
			CharLimit(MaxTextSize).
			Validate(func(data string) error {
				if data == "" {
					return ErrRequiredField
				}
				return nil
			}).
			Value(&secret.Data.Text),
	}
	return tuiFormHelper(secret, save, isAddForm, opts)
}

func tuiFormFile(secret *models.Secret, save *bool, isAddForm bool, fn func(string, string) (string, error)) *huh.Form {
	var size int
	cryptPath := secret.Data.FilePath.Path
	if cryptPath != "" {
		info, err := os.Stat(cryptPath)
		if err == nil {
			size = int(info.Size())
		}

	}
	secret.Data.FilePath.Path = ""
	opts := []huh.Field{
		huh.NewFilePicker().
			ShowHidden(true). // do not work
			ShowSize(true).
			Key("File").
			Title("File").
			Description(fmt.Sprintf("Max file size is %dM", MaxFileSize/1024/1024)).
			Value(&secret.Data.FilePath.Path),
	}
	if size != 0 {
		var show bool
		desc := fmt.Sprintf("File %s with size: %d in vault", secret.Data.FilePath.OrigName, size)
		opts = append(opts,
			huh.NewConfirm().
				DescriptionFunc(func() string {
					return desc
				}, &desc).
				Title("Show file?").
				Key("download_file").
				Validate(func(b bool) error {
					if b {
						decryptPath, err := fn(cryptPath, secret.Data.FilePath.OrigName)
						if err != nil {
							return err
						}
						secret.Data.FilePath.TmpPath = decryptPath
						desc += fmt.Sprintf("\nFile in %s\n", decryptPath)
						// сбросить нажатие кнопки
						show = false
					}
					return nil
				}).Value(&show),
		)
	}
	return tuiFormHelper(secret, save, isAddForm, opts)
}

func tuiFormHelper(secret *models.Secret, save *bool, isAddForm bool, opts []huh.Field) *huh.Form {
	options := []huh.Field{
		huh.NewInput().
			Title("Secret name").
			Key("name").
			Validate(func(data string) error {
				if data == "" {
					return ErrRequiredField
				}
				return nil
			}).
			Value(&secret.Meta.InternalMeta.SecretName),
	}
	options = append(options, opts...)
	options = append(options,
		tuiMeta(secret.Meta.Meta),
		huh.NewConfirm().
			Title("Save changes?").
			Key("save").
			Value(save).
			Validate(func(b bool) error {
				if b {
					if secret.Type == models.SecretTypeFile {
						if secret.Data.FilePath.Path == "" {
							if isAddForm {
								return fmt.Errorf("file: %w", ErrRequiredField)
							}
						} else {
							_, origName := path.Split(secret.Data.FilePath.Path)
							secret.Data.FilePath.OrigName = origName
							info, err := os.Stat(secret.Data.FilePath.Path)
							if err != nil {
								return err
							}
							if info.Size() > MaxFileSize {
								return ErrMaxFileSize
							}
						}
					}
				}
				return nil
			}),
	)
	return huh.NewForm(
		huh.NewGroup(
			options...,
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Delete secret?").
				Key("delete")).
			WithHideFunc(func() bool {
				return isAddForm || *save
			}),
	)
}

func tuiMeta(meta models.Meta) *huh.Text {
	metaStr, err := meta.PrettyString()
	if err != nil {
		logger.Logger.Error("error meta", slog.String("error", err.Error()))
	}
	return huh.NewText().
		Title("Meta info").
		CharLimit(MaxMetaSize).
		Description(fmt.Sprintf("json dict format. Max charecter %d", MaxMetaSize)).
		Validate(func(data string) error {
			clear(meta)
			if data == "" {
				return nil
			}
			err := json.Unmarshal([]byte(data), &meta)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrNotDict, err)
			}
			return nil
		}).
		Value(&metaStr).
		Key("meta")
}
