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
		// ctx := stream.Context()
		// err := contextError(stream.Context())
		// if err != nil {
		//     return err
		// }
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			code := codes.Unknown
			return status.Errorf(code, "cannot receive stream request: %v", err)
		}

		logger.Logger.Info(fmt.Sprintf("got req: %+v", req.Secret))

		res := &pb.CreateSecretResponse{
			Secret: &pb.SecretLite{
				Id:       -1,
				ServerId: 1,
				Meta: &pb.SecretDataLite{
					Id:       -1,
					ServerId: 1,
					Version:  0,
				},
				Data: &pb.SecretDataLite{
					Id:       -1,
					ServerId: 1,
					Version:  0,
				},
			},
		}
		err = stream.Send(res)
		if err != nil {
			code := codes.Unknown
			return status.Errorf(code, "cannot send stream response: %v", err)
		}
	}
	return nil
}
