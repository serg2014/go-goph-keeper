package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
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

	// TODO путь из конфига
	path, err := os.Getwd()
	if err != nil {
		return nil
	}
	tlsCreds, err := generateTLSCreds(path)
	if err != nil {
		logger.Logger.Error("failed to generate tls creds: %v", err)
	}

	// grpc server
	// создаём gRPC-сервер без зарегистрированной службы
	grpcSrv := grpc.NewServer(
		// https
		grpc.Creds(tlsCreds),
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
