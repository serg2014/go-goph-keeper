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

func Init(logdir string) error {
	// Setup logging
	f, err := os.OpenFile(path.Join(logdir, "log.json"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOpenLog, err)
	}

	Logger = slog.New(slog.NewJSONHandler(f, nil))
	RPCLogger = Logger.With("service", "gRPC/client", "component", "grpc-component")
	return nil
}

func Close() {
	f.Close()
}
