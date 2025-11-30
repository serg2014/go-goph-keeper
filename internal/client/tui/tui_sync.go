package tui

import (
	"context"
	"errors"

	"github.com/charmbracelet/huh"
	"github.com/serg2014/go-goph-keeper/internal/client/app"
)

var (
	ErrSyncMenuType = errors.New("unknown menu type")
)

type SyncMenuType int

const (
	SyncMenuTypeSyncRealTime SyncMenuType = iota
	SyncMenuTypeRegister
	SyncMenuTypeAuth
	SyncMenuTypeTest
	SyncMenuTypeMax // Must be last
)

func (s SyncMenuType) String() string {
	switch s {
	case SyncMenuTypeRegister:
		return "register new user on remoute server"
	case SyncMenuTypeAuth:
		return "auth on remote server"
	case SyncMenuTypeSyncRealTime:
		return "sync real time"
	case SyncMenuTypeTest:
		return "test uniry rpc Ping call"
	default:
		return ""
	}
}

func tuiSyncForm(ctx context.Context, app *app.ClientApp, selectedMenu SyncMenuType) error {
	switch selectedMenu {
	case SyncMenuTypeRegister:
		form := tuiFormSynRegister()
		err := form.Run()
		if err != nil {
			return err
		}

		err = app.RegisterUser(ctx, form.GetString("login"), form.GetString("password"))
		desc := ""
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Error").
					DescriptionFunc(func() string {
						if err != nil {
							return err.Error()
						} else {
							return ""
						}
					}, &desc),
			).WithHide(err == nil),
			huh.NewGroup(
				huh.NewNote().Title("User has been registred"),
			).WithHide(err != nil),
		)
		err = form.Run()
		if err != nil {
			return err
		}

	case SyncMenuTypeAuth:
		form := tuiFormSynAuth()
		err := form.Run()
		if err != nil {
			return err
		}
		err = app.AuthUser(ctx, form.GetString("login"), form.GetString("password"))
		desc := ""
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Error").
					DescriptionFunc(func() string {
						if err != nil {
							return err.Error()
						} else {
							return ""
						}
					}, &desc),
			).WithHide(err == nil),
			huh.NewGroup(
				huh.NewNote().Title("User has been authorized"),
			).WithHide(err != nil),
		)
		err = form.Run()
		if err != nil {
			return err
		}
	case SyncMenuTypeSyncRealTime:
		desc := "ok"
		err := app.Sync(ctx)
		if err != nil {
			desc = err.Error()
		}
		form := tuiFormSyncRealTime(desc)
		err = form.Run()
		return err
	case SyncMenuTypeTest:
		return app.Ping(ctx)
	default:
		return ErrSyncMenuType
	}

	return nil
}

func validateNotEmptyString(s string) error {
	if s == "" {
		return ErrRequiredField
	}
	return nil
}

func tuiFormSynRegister() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Login").
				Key("login").
				Validate(validateNotEmptyString),
			huh.NewInput().
				Title("Password").
				Key("password").
				EchoMode(huh.EchoModePassword).
				Validate(validateNotEmptyString),
			huh.NewConfirm().
				Key("save").
				Title("Register?"),
		),
	)
}

func tuiFormSynAuth() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Login").
				Key("login").
				Validate(validateNotEmptyString),
			huh.NewInput().
				Title("Password").
				Key("password").
				EchoMode(huh.EchoModePassword).
				Validate(validateNotEmptyString),
		),
	)
}

func tuiFormSyncRealTime(desc string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Realtime sync").
				Description(desc),
		),
	)
}
