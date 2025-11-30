package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/app"
	"github.com/serg2014/go-goph-keeper/internal/client/config"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
	"github.com/serg2014/go-goph-keeper/internal/client/tui"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	ErrTmpDir = errors.New("can not create tmp dir")
	ErrWrkDir = errors.New("can not create working dir")
	ErrLogDir = errors.New("can not create logs dir")
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

// interceptorLogger adapts slog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func interceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func run() error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}

	err = initWorkSpace(conf)
	if err != nil {
		return err
	}
	defer cleanWorkSpace(conf)

	// Setup logging.
	err = logger.Init(conf.LogDir(), strings.ToLower(conf.LogLevel))
	if err != nil {
		return err
	}
	defer logger.Close()
	logger.Logger.Info("start client with", slog.String("config", fmt.Sprintf("%+v", conf)))

	ctx := context.Background()
	store, err := storage.NewStorageDB(ctx, conf.DbPath())
	if err != nil {
		return err
	}

	app := app.NewApp(store, conf)

	tlsCreds, err := generateTLSCreds()
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.NewClient(
		conf.ServerAddress.String(),
		grpc.WithTransportCredentials(
			tlsCreds,
		),
		// Chain interceptors
		grpc.WithChainUnaryInterceptor(
			logging.UnaryClientInterceptor(interceptorLogger(logger.RPCLogger)),
			authClientInterceptor(app),
		),
		grpc.WithChainStreamInterceptor(
			logging.StreamClientInterceptor(interceptorLogger(logger.RPCLogger)),
			authClientStreamInterceptor(app),
		),
	)
	if err != nil {
		return err
	}
	defer conn.Close()
	// получаем переменную интерфейсного типа AuthServiceClient,
	// через которую будем отправлять сообщения
	auth := pb.NewAuthServiceClient(conn)
	keep := pb.NewGophKeeperServiceClient(conn)
	app.SetAuthClient(auth)
	app.SetKeeperClient(keep)

	err = tui.Tui(ctx, app)
	if err != nil {
		return err
	}

	return nil
}

func generateTLSCreds() (credentials.TransportCredentials, error) {
	// TODO Здесь нужно указать полный путь к файлу
	certFile := "server.crt"

	return credentials.NewClientTLSFromFile(certFile, "")
}

func initWorkSpace(c *config.Config) error {
	_, err := os.Stat(c.WorkingDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		err = os.Mkdir(c.WorkingDir, 0700)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrWrkDir, err)
		}
	}

	cleanWorkSpace(c)
	err = os.Mkdir(c.TmpDirPath(), 0700)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTmpDir, err)
	}

	err = os.Mkdir(c.LogDir(), 0700)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %w", ErrLogDir, err)
	}

	err = os.Mkdir(c.DataDir(), 0700)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %w", ErrLogDir, err)
	}

	return nil
}

func cleanWorkSpace(c *config.Config) error {
	return os.RemoveAll(c.TmpDirPath())
}
