package main

import (
	"context"
	"log"
	"strings"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/client/app"
	"github.com/serg2014/go-goph-keeper/internal/client/auth"
	"github.com/serg2014/go-goph-keeper/internal/client/config"
	"github.com/serg2014/go-goph-keeper/internal/client/storage"
	"github.com/serg2014/go-goph-keeper/internal/client/tui"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}
	defer conf.Clean()

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

	// TODO config
	conn, err := grpc.NewClient(
		"127.0.0.1:3030",
		grpc.WithTransportCredentials(
			//	insecure.NewCredentials()
			tlsCreds,
		),
		grpc.WithUnaryInterceptor(authClientInterceptor(app)),
	)
	if err != nil {
		//log.Fatal(err)
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

type clientIterceptor = func(ctx context.Context, method string, req interface{},
	reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption) error

func authClientInterceptor(app *app.ClientApp) clientIterceptor {
	return func(ctx context.Context, method string, req interface{},
		reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption) error {
		// выполняем действия перед вызовом метода
		// получить auth token и выставить мета
		if strings.HasPrefix(method, "/gophkeeper.GophKeeperService/") {
			ctx = auth.AddAuthTokenToMeta(ctx, app.GetAuthToken())
		}
		// вызываем RPC-метод
		err := invoker(ctx, method, req, reply, cc, opts...)

		// выполняем действия после вызова метода
		if err != nil && method != "/gophkeeper.AuthService/RenewAuth" {
			if e, ok := status.FromError(err); ok {
				switch e.Code() {
				case codes.Unauthenticated:
					// пробуем обновить токен
					refresh := app.GetRefreshToken()
					if refresh == "" {
						return err
					}

					ctx = auth.AddAuthTokenToMeta(ctx, refresh)
					err := app.RenewAuth(ctx)
					if err != nil {
						return err
					}
					// получить auth token и выставить мета
					ctx = auth.AddAuthTokenToMeta(ctx, app.GetAuthToken())
					// вызываем повторно RPC-метод
					return invoker(ctx, method, req, reply, cc, opts...)
				}
			}
		}
		return err
	}
}
