# Docker Startup Sequence & Service Dependencies

## Overview

This document describes the startup sequence, service dependencies, and health check configuration for the VibeCheck Docker Compose setup.

## Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Docker Network: main                      │
│                                                             │
│  ┌──────────┐         ┌──────────────┐         ┌──────────┐ │
│  │  Ollama  │────────▶│ LangExtract  │────────▶│ VibeCheck│ │
│  │  (Base)  │         │  (Service)   │         │  (MCP)   │ │
│  └──────────┘         └──────────────┘         └──────────┘ │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Startup Sequence

### 1. Ollama Service (Starts First)

**Purpose:** Base LLM inference service

**Startup Order:** First (no dependencies)

**Health Check:**
- Endpoint: TCP connectivity to port 11434
- File marker: `/tmp/ollama_ready` must exist
- Interval: 10s
- Timeout: 5s
- Retries: 3
- Start Period: 2s

**Configuration:**
```yaml
healthcheck:
  test:
    - "CMD-SHELL"
    - |
      test -f /tmp/ollama_ready && \
      bash -c '</dev/tcp/localhost/11434'
  interval: 10s
  timeout: 5s
  retries: 3
  start_period: 2s
```

**Expected Behavior:**
- Container starts and initializes Ollama
- Creates `/tmp/ollama_ready` marker when ready
- Accepts connections on port 11434
- Becomes healthy after passing health checks

---

### 2. LangExtract Service (Starts After Ollama)

**Purpose:** Language extraction and processing service

**Startup Order:** Second (depends on Ollama)

**Dependencies:**
- Requires Ollama to be healthy before starting

**Health Check:**
- Endpoint: HTTP GET `http://localhost:8000/health`
- Expected Response: 200 OK
- Interval: 10s
- Timeout: 5s
- Retries: 3
- Start Period: 10s (allows time for initialization)

**Configuration:**
```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8000/health"]
  interval: 10s
  timeout: 5s
  retries: 3
  start_period: 10s
```

**Expected Behavior:**
- Container starts after Ollama is healthy
- Connects to Ollama for language processing
- Exposes health endpoint on port 8000
- Becomes healthy after passing health checks

---

### 3. VibeCheck Service (Starts Last)

**Purpose:** MCP server for CV/JD analysis

**Startup Order:** Third (depends on Ollama and LangExtract)

**Dependencies:**
- Requires Ollama to be healthy
- Requires LangExtract to be healthy

**Health Check:**
- Endpoint: HTTP GET `http://localhost:8080/health/ready`
- Expected Response: 200 OK (when all dependencies are healthy)
- Interval: 10s
- Timeout: 5s
- Retries: 3
- Start Period: 2s

**Configuration:**
```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health/ready"]
  interval: 10s
  timeout: 5s
  retries: 3
  start_period: 2s
depends_on:
  ollama:
    condition: service_healthy
    restart: true
  langextract:
    condition: service_healthy
    restart: true
```

**Expected Behavior:**
- Container starts after both Ollama and LangExtract are healthy
- Checks storage accessibility
- Checks LangExtract connectivity
- Returns 200 OK if both are accessible
- Returns 503 if either dependency is unavailable
- Exposes MCP server on port 8080

---

## Health Check Details

### Liveness vs Readiness

**Liveness Probe (`/health/live`):**
- Always returns 200 OK
- Checks if the service is running
- No external dependencies required
- Used by Docker to determine if container should be restarted

**Readiness Probe (`/health/ready`):**
- Returns 200 OK if dependencies are accessible
- Returns 503 if dependencies are unavailable
- Checks:
  1. Storage accessibility
  2. LangExtract connectivity
- Used by Docker to determine if service should receive traffic

### Health Check Response Format

```json
{
  "status": "healthy",
  "timestamp": "2026-01-20T12:00:00Z",
  "service": "vibecheck-mcp",
  "checks": {
    "storage": "accessible",
    "langextract": "accessible"
  }
}
```

When unhealthy:
```json
{
  "status": "unhealthy",
  "timestamp": "2026-01-20T12:00:00Z",
  "service": "vibecheck-mcp",
  "checks": {
    "storage": "inaccessible",
    "langextract": "inaccessible"
  }
}
```

---

## Service Dependencies

### Dependency Graph

```
VibeCheck
├── depends_on: Ollama (condition: service_healthy, restart: true)
└── depends_on: LangExtract (condition: service_healthy, restart: true)

LangExtract
└── depends_on: Ollama (implicit - network connectivity)

Ollama
└── No dependencies (base service)
```

### Restart Behavior

All services are configured with `restart: true` for automatic recovery:
- If a dependency fails and restarts, dependent services will also restart
- Ensures consistent state across the stack
- Prevents services from running with unhealthy dependencies

---

## Environment Variables

### Ollama
- `OLLAMA_HOST`: 0.0.0.0 (binds to all interfaces)
- `OLLAMA_MODELS`: qwen3:0.6b (default model)

### LangExtract
- `LANGEXTRACT_HOST`: 0.0.0.0 (binds to all interfaces)
- `LANGEXTRACT_PORT`: 8000 (HTTP port)

### VibeCheck
- `VIBECHECK_STORAGE_PATH`: ./storage (persistent storage directory)
- `VIBECHECK_STORAGE_TTL`: 24h (document cleanup TTL)
- `OLLAMA_HOST`: ollama:11434 (service discovery via Docker DNS)
- `LANGEXTRACT_HOST`: langextract:8000 (service discovery via Docker DNS)
- `VIBECHECK_MCP_PORT`: 8080 (MCP server port, exposed to host)

---

## Network Communication

### Docker DNS Resolution

Services communicate using Docker's built-in DNS:
- `ollama:11434` → Ollama service on port 11434
- `langextract:8000` → LangExtract service on port 8000
- `vibecheck:8080` → VibeCheck service on port 8080

### Port Mapping

- **VibeCheck**: Host port 8080 → Container port 8080
  - Accessible from host: `http://localhost:8080`
  - MCP endpoint: `http://localhost:8080/mcp`
  - Health endpoints: `http://localhost:8080/health/live`, `http://localhost:8080/health/ready`

- **Ollama & LangExtract**: Internal only (not exposed to host)
  - Accessible only from within Docker network
  - Services communicate via Docker DNS

---

## Troubleshooting

### Common Startup Issues

#### 1. VibeCheck fails to start

**Symptoms:**
- VibeCheck container exits with error
- Logs show connection refused to langextract or ollama

**Diagnosis:**
```bash
# Check service status
docker compose ps

# View logs
docker compose logs vibecheck
docker compose logs langextract
docker compose logs ollama

# Check health status
docker compose exec vibecheck curl http://localhost:8080/health/ready
```

**Resolution:**
- Ensure all services are running: `docker compose up -d`
- Wait for dependencies to become healthy
- Check network connectivity between services

#### 2. LangExtract health check failing

**Symptoms:**
- LangExtract container keeps restarting
- Health check returns non-200 status

**Diagnosis:**
```bash
# Check LangExtract logs
docker compose logs langextract

# Test health endpoint manually
docker compose exec langextract curl http://localhost:8000/health

# Check Ollama connectivity
docker compose exec langextract curl http://ollama:11434/api/version
```

**Resolution:**
- Verify Ollama is healthy: `docker compose ps ollama`
- Check LangExtract initialization logs
- Ensure Ollama model is available

#### 3. Ollama health check failing

**Symptoms:**
- Ollama container keeps restarting
- `/tmp/ollama_ready` file not created

**Diagnosis:**
```bash
# Check Ollama logs
docker compose logs ollama

# Check if ready marker exists
docker compose exec ollama ls -la /tmp/ollama_ready

# Test TCP connectivity
docker compose exec ollama bash -c '</dev/tcp/localhost/11434' && echo "OK" || echo "FAIL"
```

**Resolution:**
- Increase `start_period` in health check if Ollama needs more time to initialize
- Check Ollama model download progress
- Verify Ollama is listening on correct port

#### 4. Storage inaccessible

**Symptoms:**
- VibeCheck readiness returns 503
- Logs show storage errors

**Diagnosis:**
```bash
# Check storage directory permissions
docker compose exec vibecheck ls -la /app/storage

# Check storage health
docker compose exec vibecheck curl http://localhost:8080/health/ready
```

**Resolution:**
- Ensure storage volume is mounted correctly
- Check file permissions on host system
- Verify volume exists: `docker volume ls`

### Checking Service Health

**All services:**
```bash
docker compose ps
```

**Individual service health:**
```bash
# VibeCheck readiness
curl http://localhost:8080/health/ready

# VibeCheck liveness
curl http://localhost:8080/health/live

# LangExtract health (internal only)
docker compose exec langextract curl http://localhost:8000/health

# Ollama connectivity (internal only)
docker compose exec ollama curl http://localhost:11434/api/version
```

### Recovery Procedures

#### Full Stack Restart

```bash
# Stop all services
docker compose down

# Start all services (will follow startup sequence)
docker compose up -d

# Watch logs
docker compose logs -f
```

#### Restart Individual Service

```bash
# Restart VibeCheck (will wait for dependencies)
docker compose restart vibecheck

# Restart LangExtract (will wait for Ollama)
docker compose restart langextract

# Restart Ollama (will trigger VibeCheck and LangExtract restart)
docker compose restart ollama
```

#### Manual Health Check Override

If you need to bypass health checks for debugging:

```bash
# Start without health check dependencies
docker compose up -d --no-deps vibecheck

# Or start all services ignoring health checks
docker compose up -d --force-recreate
```

---

## Production Considerations

### Resource Allocation

**Minimum Requirements:**
- Ollama: 2GB RAM, 1 CPU core
- LangExtract: 1GB RAM, 1 CPU core
- VibeCheck: 512MB RAM, 1 CPU core

**Recommended:**
- Ollama: 4GB RAM, 2 CPU cores
- LangExtract: 2GB RAM, 2 CPU cores
- VibeCheck: 1GB RAM, 2 CPU cores

### Monitoring

**Key Metrics to Monitor:**
1. Health check status (all services)
2. Container restart counts
3. Response time of `/health/ready` endpoint
4. Storage usage
5. Network connectivity between services

**Alert Conditions:**
- Service fails health checks for > 5 minutes
- Container restarts > 3 times in 10 minutes
- Readiness endpoint returns 503 for > 1 minute

### Scaling Considerations

**Current Setup:**
- Single instance of each service
- Suitable for development and small-scale production

**Future Scaling:**
- Ollama: Can scale horizontally with load balancer
- LangExtract: Stateless, can scale horizontally
- VibeCheck: Stateful (storage), requires shared storage for horizontal scaling

---

## Quick Reference

### Start Services
```bash
docker compose up -d
```

### Stop Services
```bash
docker compose down
```

### View Logs
```bash
docker compose logs -f
```

### Check Health
```bash
curl http://localhost:8080/health/ready
```

### Restart Stack
```bash
docker compose down && docker compose up -d
```

### View Startup Order
```bash
docker compose ps
# Services appear in order: ollama → langextract → vibecheck
```

---

## Version Information

**Document Version:** 1.0
**Last Updated:** 2026-01-20
**Docker Compose Version:** 2.x
**Compatible With:** VibeCheck v1.0+
