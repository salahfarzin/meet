package router

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestSetupGRPCRoutes(t *testing.T) {
	// Create a mock database connection
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	// Create a mock gRPC server
	server := grpc.NewServer()

	// This should not panic
	assert.NotPanics(t, func() {
		SetupGRPCRoutes(server, db)
	})

	// The server should have services registered
	services := server.GetServiceInfo()
	assert.Contains(t, services, "meets.MeetService")
}

func TestSetupRESTRoutes(t *testing.T) {
	ctx := context.Background()
	mux := runtime.NewServeMux()
	grpcAddr := "localhost:8080"
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// The function should not return an error immediately
	// (connection is established lazily)
	err := SetupRESTRoutes(ctx, mux, grpcAddr, opts)
	assert.NoError(t, err)
}
