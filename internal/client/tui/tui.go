package tui

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/serg2014/go-goph-keeper/internal/client/app"
)

var (
	ErrNoCommand = errors.New("no command")
	ErrQuit      = errors.New("user quit")
)

type CommandType int

const (
	CommandTypeList CommandType = iota
	CommandTypeAddSecret
	CommandTypeQuit
	CommandTypeMax // Must be last
)

func (c CommandType) String() string {
	switch c {
	case CommandTypeList:
		return "list secrets"
	case CommandTypeAddSecret:
		return "add new secret"
	case CommandTypeQuit:
		return "quit"
	default:
		return ""
	}
}

var cmds = map[CommandType]func(ctx context.Context, app *app.ClientApp) error{
	CommandTypeList:      showSecretsList,
	CommandTypeAddSecret: addSecret,
	CommandTypeQuit: func(context.Context, *app.ClientApp) error {
		fmt.Println(
			lipgloss.NewStyle().
				Width(30).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(1, 15).
				Render("Bye"),
		)
		return ErrQuit
	},
}

func Tui(ctx context.Context, app *app.ClientApp) error {
	for {
		cmd, err := intro()
		if err != nil {
			return err
		}

		err = run_command(ctx, app, *cmd)
		if err != nil {
			if errors.Is(err, ErrQuit) {
				return nil
			}
			return err
		}
	}
}

func run_command(ctx context.Context, app *app.ClientApp, command CommandType) error {
	fn, ok := cmds[command]
	if !ok {
		return fmt.Errorf("%s: %w", command, ErrNoCommand)
	}
	return fn(ctx, app)
}

func intro() (*CommandType, error) {
	opts := make([]huh.Option[CommandType], 0, CommandTypeMax)
	for i := 0; i < int(CommandTypeMax); i++ {
		str := CommandType(i).String()
		if str != "" {
			opts = append(opts, huh.NewOption(str, CommandType(i)))
		}
	}
	var cmd CommandType
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[CommandType]().
				Key("command").
				Title("Available commands").
				Options(opts...).
				Value(&cmd),
		),
	)
	err := form.Run()
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}
