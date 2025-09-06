# Appointment System Backend

This project is a backend system designed for managing appointments for psychologists and doctors using gRPC and RabbitMQ.

## Features

- gRPC-based API for handling appointment requests.
- Separate services for managing doctors and psychologists.
- Integration with RabbitMQ for message queuing.

## Project Structure

```
appointment-system-backend
├── cmd
│   └── server
│       └── main.go          # Entry point of the application
├── internal
│   ├── appointments          # Appointment management
│   │   ├── handler.go       # gRPC handler for appointments
│   │   ├── repository.go     # Data storage and retrieval for appointments
│   │   └── service.go       # Business logic for appointments
│   ├── doctors               # Doctor management
│   │   ├── handler.go       # gRPC handler for doctors
│   │   ├── repository.go     # Data storage and retrieval for doctors
│   │   └── service.go       # Business logic for doctors
│   ├── psychologists         # Psychologist management
│   │   ├── handler.go       # gRPC handler for psychologists
│   │   ├── repository.go     # Data storage and retrieval for psychologists
│   │   └── service.go       # Business logic for psychologists
│   ├── grpc                 # gRPC server setup
│   │   └── server.go        # gRPC server registration
│   └── rabbitmq             # RabbitMQ client
│       └── client.go        # RabbitMQ connection and communication
├── proto                     # Protocol buffer definitions
│   └── appointment.proto     # gRPC service and message types
├── go.mod                   # Module definition
├── go.sum                   # Module dependency checksums
└── README.md                # Project documentation
```

## Setup Instructions

1. Clone the repository:
   ```
   git clone <repository-url>
   cd appointment-system-backend
   ```

2. Install dependencies:
   ```
   go mod tidy
   ```

3. Run the application:
   ```
   go run cmd/server/main.go
   ```

## Usage

- The gRPC server will be available at the specified address and port.
- Use a gRPC client to interact with the appointment, doctor, and psychologist services.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any improvements or bug fixes.