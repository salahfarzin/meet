package router

import (
	"database/sql"

	"github.com/salahfarzin/meet/internal/availabilitytemplates"
	"github.com/salahfarzin/meet/internal/meets"
	pbAvailabilityTemplates "github.com/salahfarzin/meet/proto/availability_templates"
	pbMeets "github.com/salahfarzin/meet/proto/meets"
	"google.golang.org/grpc"
)

func SetupGRPCRoutes(server *grpc.Server, db *sql.DB, authDisabled bool) {
	meetsRepo := meets.NewRepository(db)
	templateService := availabilitytemplates.NewService(availabilitytemplates.NewRepository(db), meetsRepo)

	meetService := meets.NewService(meetsRepo, templateService)
	pbMeets.RegisterMeetServiceServer(server, meets.NewHandler(meetService))
	pbAvailabilityTemplates.RegisterAvailabilityTemplateServiceServer(server, availabilitytemplates.NewHandler(templateService, authDisabled))
}
