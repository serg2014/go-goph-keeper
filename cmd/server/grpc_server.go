package main

import (
	"context"

	pb "github.com/serg2014/go-goph-keeper/cmd/server/proto"
)

// GrpcServer struct for grpc server
type GrpcServer struct {
	pb.UnimplementedGophKeeperServiceServer
}

func (s *GrpcServer) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{}, nil
}
