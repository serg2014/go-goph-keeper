module github.com/serg2014/go-goph-keeper

go 1.24.1

replace github.com/serg2014/go-goph-keeper => ./

require (
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.3
	golang.org/x/sync v0.16.0
	google.golang.org/grpc v1.76.0
	google.golang.org/protobuf v1.36.6
)

require (
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
)
