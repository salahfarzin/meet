# System Architecture

The Meet System is built as a highly performant, scalable microservice that bridges the gap between gRPC and REST.

## Core Technologies
- **Go**: Core programming language.
- **gRPC**: Primary communication protocol for internal/high-performance service calls.
- **gRPC-Gateway**: Automatically translates gRPC into REST/JSON for web and public client compatibility.
- **MySQL**: Persistent storage for meetings and appointments.
- **JSON Metadata**: Used for storing flexible data like participant UUID lists in a structured way.

## Structure
The project follows a clean architecture pattern:

- `cmd/`: Application entry points. `cmd/api` contains both gRPC and REST server setup.
- `internal/`: Private code.
  - `meets/`: The core domain logic.
    - `handler.go`: gRPC service implementation (the "Controller").
    - `service.go`: Business logic and orchestration.
    - `repository.go`: Database abstraction and SQL queries.
- `pkg/`: Public utilities and infrastructure.
  - `db/`: Database connection management.
  - `middlewares/`: REST gateway middlewares (Auth, CORS, Logging).
  - `swagger/`: API documentation assets and embedded UI.
- `proto/`: Protocol Buffer definitions.
- `migrations/`: SQL migration files.

## Communication Flow
1. **Request**: A client sends a REST request to `:8080`.
2. **Gateway**: `grpc-gateway` receives the request, translates it to gRPC.
3. **Handler**: The `MeetsHandler` receives the gRPC call.
4. **Logic**: Handler calls the `Service`, which enforces business rules (e.g., meeting conflicts).
5. **Persistence**: `Service` calls `Repository` to interact with MySQL.
6. **Response**: The response flows back, getting translated from Proto to JSON by the gateway.

## Performance Design
- **Date Filtering**: The `GetAll` endpoint supports `from` and `to` parameters to filter meetings at the database level using indexed `start_time` and `end_time` columns.
- **UUIDs**: All entities use UUID strings for IDs to ensure compatibility with distributed systems and security (non-sequential).
