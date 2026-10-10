package router

import (
	"database/sql"
	"net/http"
	"time"

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

// usersServiceTimeout bounds each read/write of a clinic's phone assistant
// settings; the dashboard is waiting on it.
const usersServiceTimeout = 10 * time.Second

// SetupGRPCRoutes fails only on a malformed GOOYAI_ENCRYPTION_KEY or
// GOOYAI_PROXY: a set but broken value is a deploy mistake worth refusing to
// start over, unlike an unset one.
func SetupGRPCRoutes(server *grpc.Server, db *sql.DB, authDisabled bool, gooyaiCfg configs.Gooyai) error {
	sealer, err := gooyai.NewSealer(gooyaiCfg.EncryptionKey)
	if err != nil {
		return err
	}

	meetsRepo := meets.NewRepository(db)
	templateService := availabilitytemplates.NewService(availabilitytemplates.NewRepository(db), meetsRepo)

	meetService := meets.NewService(meetsRepo, templateService)
	pbMeets.RegisterMeetServiceServer(server, meets.NewHandler(meetService))
	pbAvailabilityTemplates.RegisterAvailabilityTemplateServiceServer(server, availabilitytemplates.NewHandler(templateService, authDisabled))

	specialtyService := specialties.NewService(specialties.NewRepository(db))
	pbSpecialties.RegisterSpecialtyServiceServer(server, specialties.NewHandler(specialtyService, authDisabled))

	connections := gooyai.NewUsersRepository(gooyaiCfg.UsersService, &http.Client{Timeout: usersServiceTimeout})
	clients, err := gooyai.NewClientFactoryChecked(gooyaiCfg.BaseURL, gooyaiCfg.Proxy)
	if err != nil {
		return err
	}
	gooyaiService := gooyai.NewService(connections, sealer, clients)
	pbGooyai.RegisterGooyaiServiceServer(server, gooyai.NewHandler(gooyaiService, authDisabled))
	return nil
}
