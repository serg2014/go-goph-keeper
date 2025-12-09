package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	authinterceptors "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/server/app"
	"github.com/serg2014/go-goph-keeper/internal/server/auth"
	"github.com/serg2014/go-goph-keeper/internal/server/config"
	"github.com/serg2014/go-goph-keeper/internal/server/logger"
	"github.com/serg2014/go-goph-keeper/internal/server/storage"
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
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}

	// Setup logging.
	logger.Init(conf.LogLevel)

	ctx := context.Background()
	stor, err := storage.NewStorageDB(ctx, conf.DatabaseDSN)
	if err != nil {
		return err
	}
	defer stor.Close()
	app := app.NewApp(stor, conf)

	grpcPanicRecoveryHandler := func(p any) (err error) {
		logger.RPCLogger.Error("recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}

	// TODO путь из конфига
	path, err := os.Getwd()
	if err != nil {
		return err
	}
	tlsCreds, err := generateTLSCreds(path)
	if err != nil {
		logger.Logger.Error("failed to generate tls creds: ", slog.String("error", err.Error()))
	}

	// Setup custom auth.
	authFn := func(ctx context.Context) (context.Context, error) {
		token, err := authinterceptors.AuthFromMD(ctx, "bearer")
		if err != nil {
			logger.RPCLogger.Debug("in auth interceptor no header")
			return nil, err
		}
		userID, isRefresh, err := auth.GetUserIDFromToken(token)
		if err != nil || isRefresh {
			logger.RPCLogger.Debug("in auth interceptor bad header")
			if errors.Is(err, auth.ErrTokenExpired) {
				return nil, status.Error(codes.Unauthenticated, "expired token")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid auth token")
		}
		return auth.WithUser(ctx, userID), nil
	}
	// Setup auth matcher.
	authMatcherKeeper := func(ctx context.Context, callMeta interceptors.CallMeta) bool {
		return pb.GophKeeperService_ServiceDesc.ServiceName == callMeta.Service
	}

	// Setup custom auth.
	authRefreshFn := func(ctx context.Context) (context.Context, error) {
		token, err := authinterceptors.AuthFromMD(ctx, "bearer")
		if err != nil {
			return nil, err
		}
		userID, isRefresh, err := auth.GetUserIDFromToken(token)
		if err != nil || !isRefresh {
			if errors.Is(err, auth.ErrTokenExpired) {
				return nil, status.Error(codes.Unauthenticated, "expired token")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid auth token")
		}
		// save userID in context
		return auth.WithUser(ctx, userID), nil
	}
	// Setup auth matcher.
	refreshMatcher := func(ctx context.Context, callMeta interceptors.CallMeta) bool {
		return pb.AuthService_RenewAuth_FullMethodName == callMeta.FullMethod()
	}

	// grpc server
	// создаём gRPC-сервер без зарегистрированной службы
	grpcSrv := grpc.NewServer(
		// https
		grpc.Creds(tlsCreds),
		// Chain interceptors
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(interceptorLogger(logger.RPCLogger)),
			selector.UnaryServerInterceptor(authinterceptors.UnaryServerInterceptor(authFn), selector.MatchFunc(authMatcherKeeper)),
			selector.UnaryServerInterceptor(authinterceptors.UnaryServerInterceptor(authRefreshFn), selector.MatchFunc(refreshMatcher)),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)),
		),
		grpc.ChainStreamInterceptor(
			logging.StreamServerInterceptor(interceptorLogger(logger.RPCLogger)),
			selector.StreamServerInterceptor(authinterceptors.StreamServerInterceptor(authFn), selector.MatchFunc(authMatcherKeeper)),
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)),
		),
	)
	// регистрируем сервисы
	grpcs := &GrpcServer{app: app}
	pb.RegisterAuthServiceServer(grpcSrv, grpcs)
	pb.RegisterGophKeeperServiceServer(grpcSrv, grpcs)
	reflection.Register(grpcSrv) // Enable reflection for tools like grpcurl

	// горутина обрабатывающая прерывания syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT
	go func() {
		gracefullShutdown(context.Background(), grpcSrv)
	}()

	grp := new(errgroup.Group)
	// run grpc server
	grp.Go(func() error {
		logger.Logger.Info("Try running grpc server")
		listen, err := net.Listen("tcp", conf.ServerAddress.String())
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

func gracefullShutdown(ctx context.Context, grpcSrv *grpc.Server) {
	// создаем контекст, который будет отменен при получении сигнала
	ctxS, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	select {
	// 	ждем сигнала от ОС
	case <-ctxS.Done():
		logger.Logger.Info("catch signal")
	}

	ctxT, cancelT := context.WithTimeout(ctx, waitSecBeforeShutdown)
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
}

func generateTLSCreds(path string) (credentials.TransportCredentials, error) {
	certPath := filepath.Join(path, "server.crt")
	keyPath := filepath.Join(path, "server.key")
	_, errCert := os.Stat(certPath)
	_, errKey := os.Stat(keyPath)
	if errCert != nil || errKey != nil {
		// создаём шаблон сертификата
		cert := &x509.Certificate{
			// указываем уникальный номер сертификата
			SerialNumber: big.NewInt(1658),
			// заполняем базовую информацию о владельце сертификата
			Subject: pkix.Name{
				Organization: []string{"Yandex.Praktikum"},
				Country:      []string{"RU"},
			},
			// разрешаем использование сертификата для 127.0.0.1 и ::1
			IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
			// сертификат верен, начиная со времени создания
			NotBefore: time.Now(),
			// время жизни сертификата — 10 лет
			NotAfter:     time.Now().AddDate(10, 0, 0),
			SubjectKeyId: []byte{1, 2, 3, 4, 6},
			// устанавливаем использование ключа для цифровой подписи,
			// а также клиентской и серверной авторизации
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:    x509.KeyUsageDigitalSignature,
		}

		// создаём новый приватный RSA-ключ длиной 4096 бит
		// обратите внимание, что для генерации ключа и сертификата
		// используется rand.Reader в качестве источника случайных данных
		privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, err
		}

		// создаём сертификат x.509
		certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
		if err != nil {
			return nil, err
		}

		// кодируем сертификат и ключ в формате PEM, который
		// используется для хранения и обмена криптографическими ключами
		var certPEM bytes.Buffer
		pem.Encode(&certPEM, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		})

		var privateKeyPEM bytes.Buffer
		pem.Encode(&privateKeyPEM, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})

		err = os.WriteFile(certPath, certPEM.Bytes(), 0600)
		if err != nil {
			return nil, err
		}

		err = os.WriteFile(keyPath, privateKeyPEM.Bytes(), 0600)
		if err != nil {
			return nil, err
		}
	}
	return credentials.NewServerTLSFromFile(certPath, keyPath)
}
