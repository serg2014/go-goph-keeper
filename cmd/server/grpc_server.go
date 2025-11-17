package main

import (
	"context"
	"errors"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/app"
	"github.com/serg2014/go-goph-keeper/internal/logger"
	"github.com/serg2014/go-goph-keeper/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ErrMessageEmptyLogin    = "Empty login"
	ErrMessageEmptyPassword = "Empty password"
)

// GrpcServer struct for grpc server
type GrpcServer struct {
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

func (s *GrpcServer) RegisterUser(ctx context.Context, request *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	if request.Login == "" {
		code := codes.InvalidArgument
		return nil, status.Error(code, ErrMessageEmptyLogin)
	}
	if request.Password == "" {
		code := codes.InvalidArgument
		return nil, status.Error(code, ErrMessageEmptyPassword)
	}

	// TODO use userID
	_, err := s.app.CreateUser(ctx, request.Login, request.Password)
	if err != nil {
		logger.Logger.Info("CreateUser", err.Error())
		if errors.Is(err, storage.ErrUserExists) {
			return nil, status.Error(codes.InvalidArgument, "user exists")
		}

		code := codes.Internal
		return nil, status.Error(code, code.String())
	}

	// TODO set meta
	return &pb.RegisterUserResponse{}, nil
}
