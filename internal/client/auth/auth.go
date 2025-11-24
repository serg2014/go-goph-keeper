package auth

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"
)

func AddAuthTokenToMeta(ctx context.Context, token string) context.Context {
	if token != "" {
		k := "authorization"
		v := fmt.Sprintf("%s %s", "bearer", token)
		md, ok := metadata.FromOutgoingContext(ctx)
		if ok {
			md.Set(k, v)
		} else {
			md = metadata.New(map[string]string{k: v})
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}
