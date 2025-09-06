package grpc

import (
	"log"
	"net"

	"github.com/salahfarzin/appointment/internal/appointments"
	"github.com/salahfarzin/appointment/internal/doctors"
	"github.com/salahfarzin/appointment/internal/psychologists"

	"google.golang.org/grpc"
)

type Server struct {
	grpcServer *grpc.Server
}

func NewServer() *Server {
	return &Server{
		grpcServer: grpc.NewServer(),
	}
}

func (s *Server) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	appointments.RegisterAppointmentServiceServer(s.grpcServer, appointments.NewAppointmentHandler())
	doctors.RegisterDoctorServiceServer(s.grpcServer, doctors.NewDoctorHandler())
	psychologists.RegisterPsychologistServiceServer(s.grpcServer, psychologists.NewPsychologistHandler())

	log.Printf("gRPC server listening on %s", address)
	return s.grpcServer.Serve(listener)
}

func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}
