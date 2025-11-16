package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/logger"
)

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/keeper.proto

// waitSecBeforeShutdown how many seconds wait before force shutdown
const waitSecBeforeShutdown = 5 * time.Second

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
	// Setup logging.
	logger.Init()

	grpcPanicRecoveryHandler := func(p any) (err error) {
		logger.RPCLogger.Error("recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}
	// grpc server
	// создаём gRPC-сервер без зарегистрированной службы
	grpcSrv := grpc.NewServer(
		// Chain interceptors
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(interceptorLogger(logger.RPCLogger)),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)),
		),
	)
	// регистрируем сервис
	pb.RegisterGophKeeperServiceServer(grpcSrv, &GrpcServer{})
	reflection.Register(grpcSrv) // Enable reflection for tools like grpcurl

	var wg sync.WaitGroup
	wg.Add(1)
	// горутина обрабатывающая прерывания syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT
	go func() {
		defer wg.Done()
		// создаем контекст, который будет отменен при получении сигнала
		ctxS, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		defer stop()

		select {
		// 	ждем сигнала от ОС
		case <-ctxS.Done():
			logger.Logger.Info("catch signal")
		}

		ctxT, cancelT := context.WithTimeout(context.Background(), waitSecBeforeShutdown)
		defer cancelT()

		grp := new(errgroup.Group)
		grp.Go(func() error {
			logger.Logger.Info("Gracefull shutdown grpc server")
			// GracefulStop блокирующая операция
			grpcSrv.GracefulStop()
			// отменяем таймаут
			cancelT()
			return nil
		})
		grp.Go(func() error {
			<-ctxT.Done()
			if ctxT.Err() == context.DeadlineExceeded {
				logger.Logger.Info("Force shutdown grpc server")
				grpcSrv.Stop()
			}
			return nil
		})
		// ожидаем завершения работы сервера
		grp.Wait()
	}()

	grp := new(errgroup.Group)
	// run grpc server
	grp.Go(func() error {
		logger.Logger.Info("Try running grpc server")
		listen, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "localhost", 3030))
		if err != nil {
			return err
		}
		if err := grpcSrv.Serve(listen); err != nil {
			return err
		}
		return nil
	})

	// ожидаем завершения работы сервера
	if err := grp.Wait(); err != nil {
		return err
	}
	return nil
}
