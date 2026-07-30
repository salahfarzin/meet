package router

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pbAvailabilityTemplates "github.com/salahfarzin/meet/proto/availability_templates"
	pbMeets "github.com/salahfarzin/meet/proto/meets"
	"google.golang.org/grpc"
)

func SetupRESTRoutes(ctx context.Context, mux *runtime.ServeMux, grpcAddr string, opts []grpc.DialOption) error {
	if err := pbMeets.RegisterMeetServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return err
	}
	if err := pbAvailabilityTemplates.RegisterAvailabilityTemplateServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return err
	}
	return nil
}
