package router

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pbAvailabilityTemplates "github.com/salahfarzin/meet/proto/availability_templates"
	pbMeets "github.com/salahfarzin/meet/proto/meets"
	pbSpecialties "github.com/salahfarzin/meet/proto/specialties"
	"google.golang.org/grpc"
)

// SetupRESTRoutes registers MeetService first: the gateway mux matches the
// most recently registered pattern first, so the literal /meets/specialties
// and /meets/availability-templates routes win over MeetService's /meets/{uuid}.
func SetupRESTRoutes(ctx context.Context, mux *runtime.ServeMux, grpcAddr string, opts []grpc.DialOption) error {
	if err := pbMeets.RegisterMeetServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return err
	}
	if err := pbAvailabilityTemplates.RegisterAvailabilityTemplateServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return err
	}
	if err := pbSpecialties.RegisterSpecialtyServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		return err
	}
	return nil
}
