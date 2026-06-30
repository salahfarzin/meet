package router

import (
	"database/sql"

	"github.com/salahfarzin/meet/internal/meets"
	pbMeets "github.com/salahfarzin/meet/proto/meets"
	"google.golang.org/grpc"
)

func SetupGRPCRoutes(server *grpc.Server, db *sql.DB) {
	// Task 4 will wire the real identity.Client from USER_SERVICE config.
	// Pass nil here; service.ListScheduling is nil-safe for the identity field.
	meetService := meets.NewService(meets.NewRepository(db), nil)
	pbMeets.RegisterMeetServiceServer(server, meets.NewHandler(meetService))
}
