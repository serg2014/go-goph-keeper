package main

import (
	"context"
	"errors"
	"log/slog"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
	"github.com/serg2014/go-goph-keeper/internal/server/auth"
	"github.com/serg2014/go-goph-keeper/internal/server/logger"
	"github.com/serg2014/go-goph-keeper/internal/server/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

	jwt, err := auth.BuildAccessJWT(userID)
	if err != nil {
		return nil, err
	}
	jwtRefresh, err := auth.BuildRefreshJWT(userID)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterUserResponse{
		Access:  &pb.JWTToken{Token: jwt},
		Refresh: &pb.JWTToken{Token: jwtRefresh},
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

	jwt, err := auth.BuildAccessJWT(userID)
	if err != nil {
		return nil, err
	}
	jwtRefresh, err := auth.BuildRefreshJWT(userID)
	if err != nil {
		return nil, err
	}

	return &pb.AuthUserResponse{
		Access:  &pb.JWTToken{Token: jwt},
		Refresh: &pb.JWTToken{Token: jwtRefresh},
	}, nil
}

func (s *GrpcServer) RenewAuth(ctx context.Context, request *pb.RenewAuthRequest) (*pb.RenewAuthResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	jwt, err := auth.BuildAccessJWT(userID)
	if err != nil {
		return nil, err
	}
	jwtRefresh, err := auth.BuildRefreshJWT(userID)
	if err != nil {
		return nil, err
	}

	return &pb.RenewAuthResponse{
		Access:  &pb.JWTToken{Token: jwt},
		Refresh: &pb.JWTToken{Token: jwtRefresh},
	}, nil
}
