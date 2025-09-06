package router

import (
	"database/sql"

	"github.com/salahfarzin/meet/internal/meets"
	pbMeets "github.com/salahfarzin/meet/proto/meets"
	"google.golang.org/grpc"
)

func SetupGRPCRoutes(server *grpc.Server, db *sql.DB) {
	meetService := meets.NewService(meets.NewRepository(db))
	pbMeets.RegisterMeetServiceServer(server, meets.NewHandler(meetService))
}
