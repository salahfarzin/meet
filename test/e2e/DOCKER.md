# Running E2E Tests with Docker

This guide explains how to run E2E tests using Docker containers for complete isolation.

## 🐳 **Docker Setup**

The `docker-compose.test.yml` file creates an isolated test environment with:
- **MySQL Test Database** (port 3307, tmpfs for speed)
- **Database Migrations** (automatic)
- **Application in Test Mode** (port 8081)

## 📋 **Available Commands**

### **Option 1: Fully Automated** (Recommended)

```bash
# Run tests with automatic setup and teardown
make test-e2e-docker-clean
```

**What it does:**
1. 🧹 Stops and removes any existing test containers
2. 🐳 Starts fresh MySQL, runs migrations, starts app
3. ⏳ Waits for services to be ready
4. 🧪 Runs all E2E tests
5. 🛑 Stops and removes all containers
6. ✅ Reports results

**Use when:** You want a completely fresh test run every time.

---

### **Option 2: Reuse Containers** (Faster)

```bash
# Run tests with existing containers (or create if needed)
make test-e2e-docker
```

**What it does:**
1. 🐳 Starts containers (reuses if already running)
2. ⏳ Waits for services
3. 🧪 Runs E2E tests
4. 🛑 Stops containers (keeps data)

**Use when:** Running tests multiple times and want to save startup time.

---

### **Option 3: Manual Control**

```bash
# Start test environment
make test-e2e-docker-up

# Run tests manually (can run multiple times)
APP_URL=http://localhost:8081 make test-e2e

# Stop when done
make test-e2e-docker-down
```

**Use when:** You want to run tests multiple times or debug issues.

---

## 🎯 **Quick Start**

```bash
# First time - clean run
make test-e2e-docker-clean

# Subsequent runs - faster
make test-e2e-docker
```

---

## 🔧 **Port Configuration**

| Service | Port | Why Different? |
|---------|------|----------------|
| MySQL Test | 3307 | Avoid conflict with production MySQL (3306) |
| App Test | 8081 | Avoid conflict with production app (8080/8083) |
| gRPC Test | 50053 | Avoid conflict with production gRPC (50052/9093) |

---

## 📊 **Test Environment Details**

### **Database**
- **Image**: `mysql:8.0`
- **Database**: `meet_db_test`
- **User**: `meet_user`
- **Password**: `root`
- **Storage**: tmpfs (in-memory, fast, not persisted)

### **Application**
- **Mode**: `APP_ENV=test` (authentication bypassed)
- **Port**: `8081`
- **Logs**: `./storage/logs`

---

## 🐛 **Troubleshooting**

### **"Port already in use"**
```bash
# Check what's using the port
lsof -i :8081

# Stop all test containers
make test-e2e-docker-down

# Or stop all Docker containers
docker-compose -f docker-compose.test.yml down -v
```

### **"Tests failing with connection refused"**
```bash
# Services might not be ready yet
# Increase wait time in Makefile (change sleep 10 to sleep 15)

# Or check service health
docker-compose -f docker-compose.test.yml ps
docker-compose -f docker-compose.test.yml logs app-test
```

### **"Database has old data"**
```bash
# Use clean command to start fresh
make test-e2e-docker-clean
```

---

## 🚀 **CI/CD Integration**

For GitHub Actions or other CI:

```yaml
- name: Run E2E tests with Docker
  run: make test-e2e-docker-clean
```

The `test-e2e-docker-clean` command is perfect for CI because it:
- ✅ Starts with a clean state
- ✅ Handles all setup automatically
- ✅ Cleans up after itself
- ✅ Returns proper exit codes

---

## 📝 **Comparison: Docker vs Local**

| Aspect | Docker | Local |
|--------|--------|-------|
| **Setup** | Automatic | Manual |
| **Isolation** | Complete | Shared DB |
| **Speed** | Slower startup | Faster startup |
| **Cleanup** | Automatic | Manual |
| **CI/CD** | Perfect | Difficult |
| **Debugging** | Harder | Easier |

**Recommendation:**
- **Development**: Use local (`make test-e2e`)
- **CI/CD**: Use Docker (`make test-e2e-docker-clean`)
- **Clean runs**: Use Docker (`make test-e2e-docker-clean`)

---

## 🎓 **Examples**

### **Development Workflow**
```bash
# Start test environment once
make test-e2e-docker-up

# Run tests multiple times while developing
APP_URL=http://localhost:8081 make test-e2e
APP_URL=http://localhost:8081 make test-e2e
APP_URL=http://localhost:8081 make test-e2e

# Stop when done
make test-e2e-docker-down
```

### **Quick Test Run**
```bash
# One command - fresh environment
make test-e2e-docker-clean
```

### **Debugging**
```bash
# Start environment
make test-e2e-docker-up

# Check logs
docker-compose -f docker-compose.test.yml logs -f app-test

# Run specific test
APP_URL=http://localhost:8081 go test -v ./test/e2e/... -run TestMeetsAPI_CreateMeet

# Stop
make test-e2e-docker-down
```

---

## ✅ **Success Criteria**

When everything is working, you should see:

```
🐳 Starting fresh test environment...
⏳ Waiting for services to be ready...
🧪 Running E2E tests...
=== RUN   TestMeetsAPI_CreateMeet
=== RUN   TestMeetsAPI_GetOneMeet
=== RUN   TestMeetsAPI_GetAllMeets
=== RUN   TestMeetsAPI_UpdateMeet
=== RUN   TestMeetsAPI_DeleteMeet
=== RUN   TestMeetsAPI_FullCRUDFlow
--- PASS: TestMeetsAPI_CreateMeet (0.05s)
--- PASS: TestMeetsAPI_GetOneMeet (0.03s)
--- PASS: TestMeetsAPI_GetAllMeets (0.04s)
--- PASS: TestMeetsAPI_UpdateMeet (0.03s)
--- PASS: TestMeetsAPI_DeleteMeet (0.02s)
--- PASS: TestMeetsAPI_FullCRUDFlow (0.15s)
PASS
🛑 Cleaning up...
✅ E2E tests completed and cleaned up
```

---

## 🔗 **Related Commands**

```bash
# View all test-related commands
make help | grep test

# Available commands:
# make test                  - Run unit tests
# make test-e2e              - Run E2E tests (app must be running)
# make test-e2e-with-app     - Run E2E tests with automatic app lifecycle
# make test-e2e-docker       - Run E2E tests with Docker (reuse containers)
# make test-e2e-docker-clean - Run E2E tests with Docker (fresh start)
# make test-e2e-docker-up    - Start Docker test environment
# make test-e2e-docker-down  - Stop Docker test environment
```
