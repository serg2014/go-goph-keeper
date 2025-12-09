package logger

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger
var RPCLogger *slog.Logger

func Init(logLevel string) {
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
	// Setup logging.
	Logger = slog.New(slog.NewJSONHandler(os.Stderr, opts))
	RPCLogger = Logger.With("service", "gRPC/server", "component", "grpc-component")
}
