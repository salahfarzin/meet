package router

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pbMeets "github.com/salahfarzin/meet/proto/meets"
	"google.golang.org/grpc"
)

func SetupRESTRoutes(ctx context.Context, mux *runtime.ServeMux, grpcAddr string, opts []grpc.DialOption) error {
	err := pbMeets.RegisterMeetServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts)
	if err != nil {
		return err
	}
	return nil
}
