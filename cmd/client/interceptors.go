package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/serg2014/go-goph-keeper/internal/client/app"
	"github.com/serg2014/go-goph-keeper/internal/client/auth"
	"github.com/serg2014/go-goph-keeper/internal/client/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authClientInterceptor(app *app.ClientApp) grpc.UnaryClientInterceptor {
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
					ctx, err := app.RenewAuth(ctx)
					if err != nil {
						return err
					}
					// вызываем повторно RPC-метод
					return invoker(ctx, method, req, reply, cc, opts...)
				}
			}
		}
		return err
	}
}

func authClientStreamInterceptor(app *app.ClientApp) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		logger.Logger.Info("before stream")
		// выполняем действия перед вызовом метода
		// получить auth token и выставить мета
		if strings.HasPrefix(method, "/gophkeeper.GophKeeperService/") {
			ctx = auth.AddAuthTokenToMeta(ctx, app.GetAuthToken())
		}

		// вызываем RPC-метод
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, err
		}
		logger.Logger.Info("check metadata")
		return &wrappedStream{clientStream}, nil

		// // выполняем действия после вызова метода
		// if err != nil && method != "/gophkeeper.AuthService/RenewAuth" {
		// 	if e, ok := status.FromError(err); ok {
		// 		switch e.Code() {
		// 		case codes.Unauthenticated:
		// 			// получаем рефреш токен
		// 			refresh := app.GetRefreshToken()
		// 			if refresh == "" {
		// 				return clientStream, err
		// 			}

		// 			ctx = auth.AddAuthTokenToMeta(ctx, refresh)
		// 			// пробуем обновить токен
		// 			err := app.RenewAuth(ctx)
		// 			if err != nil {
		// 				return clientStream, err
		// 			}
		// 			// получить auth token и выставить мета
		// 			ctx = auth.AddAuthTokenToMeta(ctx, app.GetAuthToken())
		// 			// вызываем повторно RPC-метод
		// 			return streamer(ctx, desc, cc, method, opts...)
		// 		}
		// 	}
		// }
		// return clientStream, err
	}
}

type wrappedStream struct {
	grpc.ClientStream
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	logger.Logger.Debug(fmt.Sprintf("--> Sending message: %v", m))
	err := w.ClientStream.SendMsg(m)
	if err != nil {
		logger.Logger.Debug(fmt.Sprintf("--> SendMsg failed: %v", err))
	}
	return err
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	err := w.ClientStream.RecvMsg(m)
	if err != nil {
		logger.Logger.Debug(fmt.Sprintf("--> RecvMsg failed: %v", err))
	} else {
		logger.Logger.Debug(fmt.Sprintf("--> Received message: %v", m))
	}
	return err
}
