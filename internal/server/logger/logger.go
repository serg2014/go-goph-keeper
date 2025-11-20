package logger

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger
var RPCLogger *slog.Logger

func Init() {
	// Setup logging.
	Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
	RPCLogger = Logger.With("service", "gRPC/server", "component", "grpc-component")
}
