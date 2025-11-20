package main

import (
	"context"
	"errors"
	"log/slog"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/app"
	"github.com/serg2014/go-goph-keeper/internal/auth"
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

func (s *GrpcServer) RegisterUser(ctx context.Context, request *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	if request.Login == "" {
		code := codes.InvalidArgument
		return nil, status.Error(code, ErrMessageEmptyLogin)
	}
	if request.Password == "" {
		code := codes.InvalidArgument
		return nil, status.Error(code, ErrMessageEmptyPassword)
	}

	userID, err := s.app.CreateUser(ctx, request.Login, request.Password)
	if err != nil {
		logger.Logger.Info("CreateUser", slog.String("error", err.Error()))
		if errors.Is(err, storage.ErrUserExists) {
			return nil, status.Error(codes.InvalidArgument, "user exists")
		}

		code := codes.Internal
		return nil, status.Error(code, code.String())
	}

	jwt, err := auth.BuildJWTString(userID)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterUserResponse{
		Token: &pb.JWTToken{Token: jwt},
	}, nil
}

func (s *GrpcServer) AuthUser(ctx context.Context, request *pb.AuthUserRequest) (*pb.AuthUserResponse, error) {
	if request.Login == "" {
		code := codes.InvalidArgument
		return nil, status.Error(code, ErrMessageEmptyLogin)
	}
	if request.Password == "" {
		code := codes.InvalidArgument
		return nil, status.Error(code, ErrMessageEmptyPassword)
	}

	userID, err := s.app.GetUser(ctx, request.Login, request.Password)
	if err != nil {
		if errors.Is(err, storage.ErrUserOrPassword) {
			return nil, status.Error(codes.InvalidArgument, "bad user or password")
		}
		logger.Logger.Error("AuthUser", slog.String("error", err.Error()))
		code := codes.Internal
		return nil, status.Error(code, code.String())
	}

	jwt, err := auth.BuildJWTString(userID)
	if err != nil {
		return nil, err
	}

	return &pb.AuthUserResponse{
		Token: &pb.JWTToken{Token: jwt},
	}, nil
}
