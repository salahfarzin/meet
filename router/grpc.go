package router

import (
	"database/sql"

	"github.com/salahfarzin/meet/configs"
	"github.com/salahfarzin/meet/internal/availabilitytemplates"
	"github.com/salahfarzin/meet/internal/gooyai"
	"github.com/salahfarzin/meet/internal/meets"
	"github.com/salahfarzin/meet/internal/specialties"
	pbAvailabilityTemplates "github.com/salahfarzin/meet/proto/availability_templates"
	pbGooyai "github.com/salahfarzin/meet/proto/gooyai"
	pbMeets "github.com/salahfarzin/meet/proto/meets"
	pbSpecialties "github.com/salahfarzin/meet/proto/specialties"
	"google.golang.org/grpc"
)

func SetupGRPCRoutes(server *grpc.Server, db *sql.DB, authDisabled bool, gooyaiCfg configs.Gooyai) {
	meetsRepo := meets.NewRepository(db)
	templateService := availabilitytemplates.NewService(availabilitytemplates.NewRepository(db), meetsRepo)

	meetService := meets.NewService(meetsRepo, templateService)
	pbMeets.RegisterMeetServiceServer(server, meets.NewHandler(meetService))
	pbAvailabilityTemplates.RegisterAvailabilityTemplateServiceServer(server, availabilitytemplates.NewHandler(templateService, authDisabled))

	specialtyService := specialties.NewService(specialties.NewRepository(db))
	pbSpecialties.RegisterSpecialtyServiceServer(server, specialties.NewHandler(specialtyService, authDisabled))

	gooyaiService := gooyai.NewService(gooyaiCfg.TenantKeys, gooyai.NewClientFactory(gooyaiCfg.BaseURL, gooyaiCfg.Env))
	pbGooyai.RegisterGooyaiServiceServer(server, gooyai.NewHandler(gooyaiService, authDisabled))
}
