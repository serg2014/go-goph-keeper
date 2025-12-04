package main

import (
	"context"
	"fmt"
	"io"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/server/app"
	"github.com/serg2014/go-goph-keeper/internal/server/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ErrMessageEmptyLogin    = "Empty login"
	ErrMessageEmptyPassword = "Empty password"
)

// GrpcServer struct for grpc server
type GrpcServer struct {
	pb.UnimplementedAuthServiceServer
	pb.UnimplementedGophKeeperServiceServer

	app *app.MyApp
}

func (s *GrpcServer) Ping(ctx context.Context, request *pb.PingRequest) (*pb.PingResponse, error) {
	err := s.app.Ping(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.PingResponse{}, nil
}

func (s *GrpcServer) CreateSecrets(stream grpc.BidiStreamingServer[pb.CreateSecretRequest, pb.CreateSecretResponse]) error {
	for {
		ctx := stream.Context()
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot receive stream request: %v", err)
		}

		logger.Logger.Debug(fmt.Sprintf("got req: %+v", req.Secret))

		if req.File != nil {
			// вычитываем данные файла и секрета в отдельном методе
			err = s.app.CreateFileSecret(ctx, stream, req)
			if err != nil {
				return err
			}
			continue
		}

		res, err := s.app.CreateSecret(ctx, req)
		if err != nil {
			return err
		}
		err = stream.Send(res)
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot send stream response: %v", err)
		}
	}
}

func (s *GrpcServer) UpdateSecrets(stream grpc.BidiStreamingServer[pb.UpdateSecretRequest, pb.UpdateSecretResponse]) error {
	for {
		ctx := stream.Context()
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot receive stream request: %v", err)
		}

		logger.Logger.Info(fmt.Sprintf("got req: %+v", req))

		if req.File != nil {
			// вычитываем данные файла и секрета в отдельном методе
			err = s.app.UpdateFileSecret(ctx, stream, req)
			if err != nil {
				return err
			}
			continue
		}

		res, err := s.app.UpdateSecret(ctx, req)
		if err != nil {
			return err
		}

		err = stream.Send(res)
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot send stream response: %v", err)
		}
	}
}

func (s *GrpcServer) DeleteSecrets(stream grpc.BidiStreamingServer[pb.DeleteSecretRequest, pb.DeleteSecretResponse]) error {
	for {
		ctx := stream.Context()
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot receive stream request: %v", err)
		}

		logger.Logger.Info(fmt.Sprintf("got req: %+v", req))

		res, err := s.app.DeleteSecret(ctx, req)
		if err != nil {
			return err
		}

		err = stream.Send(res)
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot send stream response: %v", err)
		}
	}
}

func (s *GrpcServer) SecretsListInfo(request *pb.SecretsListRequest, srv grpc.ServerStreamingServer[pb.SecretsListResponse]) error {
	ctx := srv.Context()
	list, err := s.app.GetSecretsListInfo(ctx)
	if err != nil {
		return err
	}
	for _, item := range list {
		if err := srv.Send(item); err != nil {
			return err
		}
	}
	return nil
}

func (s *GrpcServer) GetSecrets(stream grpc.BidiStreamingServer[pb.GetSecretsRequest, pb.GetSecretsResponse]) error {
	for {
		ctx := stream.Context()
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot receive stream request: %v", err)
		}

		logger.Logger.Info(fmt.Sprintf("got req: %+v", req))

		res, err := s.app.GetSecret(ctx, req)
		if err != nil {
			return err
		}

		err = stream.Send(res)
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot send stream response: %v", err)
		}

		// TODO отправка файла
	}
}
