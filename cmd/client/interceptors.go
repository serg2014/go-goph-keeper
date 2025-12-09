package main

import (
	"context"
	"fmt"
	"log/slog"
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
		// Оборачиваем стандартный поток в кастомный, чтобы перехватить ошибки после начала потока
		return &wrappedStream{
			ClientStream: clientStream,
			auth:         app,
			ctx:          ctx,
			method:       method,
			cc:           cc,
			streamer:     streamer,
			desc:         desc,
			opts:         opts,
		}, nil
	}
}

type authManager interface {
	RenewAuth(ctx context.Context) (context.Context, error)
}

type wrappedStream struct {
	grpc.ClientStream
	auth     authManager
	ctx      context.Context
	method   string
	cc       *grpc.ClientConn
	streamer grpc.Streamer
	desc     *grpc.StreamDesc
	opts     []grpc.CallOption
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	logger.Logger.Debug(fmt.Sprintf("<-- Sending message: %v", m))
	err := w.ClientStream.SendMsg(m)
	if err != nil {
		logger.Logger.Debug(fmt.Sprintf("<-- SendMsg failed: %v", err))
	}
	return w.handleError(err, true)
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	err := w.ClientStream.RecvMsg(m)
	if err != nil {
		logger.Logger.Debug(fmt.Sprintf("--> RecvMsg failed: %v", err))
	} else {
		logger.Logger.Debug(fmt.Sprintf("--> Received message: %v", m))
	}
	return w.handleError(err, false)
}

func (w *wrappedStream) handleError(err error, isSend bool) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	// Если ошибка не связана с авторизацией, просто возвращаем ее
	if st.Code() != codes.Unauthenticated {
		return err
	}

	// пробуем обновить токен
	ctx, err := w.auth.RenewAuth(w.ctx)
	if err != nil {
		logger.Logger.Debug("can not renew token")
		return err
	}

	// Вызываем оригинальный streamer еще раз с новым контекстом
	newStream, err := w.streamer(ctx, w.desc, w.cc, w.method, w.opts...)
	if err != nil {
		logger.Logger.Debug(fmt.Sprintf("can not reconnect stream: %v", err))
		return err
	}

	// Заменяем текущий стрим новым
	w.ClientStream = newStream

	// Если мы обрабатывали ошибку при SendMsg, мы не можем автоматически переотправить сообщение M,
	// так как мы его не буферизировали. Для сложных случаев (BiDi/Client-streaming)
	// этот интерцептор поможет лишь установить соединение с новым токеном, но само сообщение придется отправлять
	// повторно в коде приложения.

	// Для Server-side streaming (RecvMsg) это работает лучше.

	logger.Logger.Info("reconnect stream. A repeat operation is not guaranteed.", slog.Bool("isSend", isSend))

	return auth.ErrNeedRetry

	// В большинстве случаев мы не можем автоматически повторить SendMsg,
	// просто возвращаем nil, чтобы приложение продолжило работу с новым стримом.
	// Если это RecvMsg, то следующий вызов RecvMsg получит данные уже из нового стрима.
	// return nil
}
