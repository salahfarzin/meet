# Meet System Backend

This project is a backend system designed for managing meets for psychologists and doctors using gRPC and RabbitMQ.

## Features

- gRPC-based API for handling meet requests.
- Separate services for managing doctors and psychologists.
- Integration with RabbitMQ for message queuing.

## REST API
```
curl http://localhost:8080/meets
```
## GRPC API
```
grpcurl -plaintext -d '{"meet": {"title":"Test","start":"2025-09-10 10:25"}}' localhost:50051 meets.MeetService/CreateMeet
```

## Build proto for meet (with support RESTful API)
```
protoc -I. -I./googleapis --go_out=proto --go-grpc_out=proto --grpc-gateway_out=proto proto/meets/meets.proto 
```

## Database migrations
the most popular tool for generating and running database migrations is golang-migrate/migrate.

Migration path: ```database/migrations```
### Install migrate CLI (if you haven’t already):
```
brew install golang-migrate
```
### Create migrations
```
migrate create -ext sql -dir database/migrations -seq create_meets_table
```
### Run migration
```
migrate -path database/migrations -database \"mysql://user:password@tcp(localhost:3306)/dbname\" up
```