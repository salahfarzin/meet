package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/salahfarzin/appointment/configs"
	"google.golang.org/grpc"

	pb "github.com/salahfarzin/appointment/proto"
)

type server struct {
	pb.UnimplementedAppointmentServiceServer
}

// Example method
func (s *server) GetAppointments(ctx context.Context, req *pb.GetAppointmentRequest) (*pb.GetAppointmentResponse, error) {
	return &pb.GetAppointmentResponse{
		Appointments: []*pb.Appointment{
			{Id: 1, Title: "Dentist", Date: "2025-08-20"},
		},
	}, nil
}

func loadConfig() *configs.Configs {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
	cfg := configs.New()

	portStr := os.Getenv("APP_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Invalid APP_PORT: %v", err)
	}
	cfg.Port = port

	cfg.Mysql = &mysql.Config{
		User:                 os.Getenv("DB_USER"),
		Passwd:               os.Getenv("DB_PASSWORD"),
		Net:                  "tcp",
		Addr:                 os.Getenv("DB_URL"), // e.g., "127.0.0.1:3306"
		DBName:               os.Getenv("DB_NAME"),
		AllowNativePasswords: true,
		ParseTime:            true,
	}
	return cfg
}

func startGRPCServer(srv pb.AppointmentServiceServer, address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterAppointmentServiceServer(grpcServer, srv)
	log.Printf("Starting gRPC server on %s", address)
	return grpcServer.Serve(lis)
}

func main() {
	cfg := loadConfig()
	fmt.Println(cfg)

	address := fmt.Sprintf(":%s", cfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := startGRPCServer(&server{}, address); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down gracefully")
}
