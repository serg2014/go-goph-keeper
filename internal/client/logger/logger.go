package logger

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
)

var (
	ErrOpenLog = errors.New("can not open log file")
)

var Logger *slog.Logger
var RPCLogger *slog.Logger
var f *os.File

func Init(logdir string, logLevel string) error {
	// Setup logging
	var err error
	f, err = os.OpenFile(path.Join(logdir, "log.json"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOpenLog, err)
	}
	var level slog.Leveler
	switch logLevel {
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "debug":
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	Logger = slog.New(slog.NewJSONHandler(f, opts))
	RPCLogger = Logger.With("service", "gRPC/client", "component", "grpc-component")
	return nil
}

func Close() {
	if f != nil {
		f.Close()
	}
}
