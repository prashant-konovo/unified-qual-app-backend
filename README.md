# Unified Qual API — Go + Chi Backend

Backend API for the Unified Qualitative Research platform.  
Go 1.24 + Chi v5. Deployed to EKS (data-qa) via ArgoCD.

## Tech Stack
- **Go 1.24** + **Chi v5** router
- **robfig/cron/v3** — 9 scheduled jobs
- MySQL (IRIS + QS-Tool)
- 11 external service integrations (Conference, Notification, GCal, Stripe, Tango, PayPal, Bandwidth, Decipher, CastingWords, EventLog, GoogleSheets)
- Docker multi-stage build → ECR → EKS

## Local Development (data-qa)

### Prerequisites
- Go 1.24+
- AWS CLI v2 with `cli-prashant-singh-konovo` profile (or equivalent IAM access)
- [AWS SSM Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
- `kubectl` configured for `eks-cluster-apps-use2-konovo-dev`

### 1. Start SSM Tunnels (3 terminals)

The data-qa databases are in a private VPC (`vpc-7ae31207`, us-east-1). SSM port-forwarding through a dedicated EC2 instance (`i-097a5b6d0bf443137`) provides local access.

```bash
# Terminal 1: IRIS Primary → localhost:13306
aws ssm start-session --target i-097a5b6d0bf443137 --region us-east-1 \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters '{"host":["172.31.7.31"],"portNumber":["3306"],"localPortNumber":["13306"]}'

# Terminal 2: IRIS Read-Only → localhost:13307
aws ssm start-session --target i-097a5b6d0bf443137 --region us-east-1 \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters '{"host":["172.31.39.44"],"portNumber":["3306"],"localPortNumber":["13307"]}'

# Terminal 3: QS MySQL → localhost:13308
aws ssm start-session --target i-097a5b6d0bf443137 --region us-east-1 \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters '{"host":["172.31.39.18"],"portNumber":["3306"],"localPortNumber":["13308"]}'
```

> **SSM idle timeout** is set to **60 minutes** (default was 20 min).  
> This was configured in SSM Session Manager Preferences (`SSM-SessionManagerRunShell` document v2, `idleSessionTimeout: "60"`).  
> If a tunnel still drops, re-run the command.

#### Keepalive (prevents idle timeout)

Run a background loop that pings all tunnel ports every 5 minutes to reset the idle timer:

```bash
while true; do
  nc -z -w2 localhost 13306
  nc -z -w2 localhost 13307
  nc -z -w2 localhost 13308
  echo "keepalive ping sent"
  sleep 300
done
```

> **Tip:** Run this in a spare terminal alongside the 3 tunnel terminals. The TCP connect (`nc -z`) generates enough traffic to prevent SSM from marking the session as idle.

### 2. Source Environment & Run

```bash
source .env.local   # Loads data-qa credentials + tunneled DB ports
make run             # Starts on :8080
```

### 3. Verify

```bash
curl http://localhost:8080/health
# Expected: {"checks":{"incrowdDB":"ok","incrowdRODB":"ok","qstoolDB":"ok"},"environment":"local","status":"healthy"}
```

### Environment File (`.env.local`)

Pre-configured with data-qa credentials. Key overrides for local:

| Variable | Local Value | Purpose |
|----------|-------------|---------|
| `ENVIRONMENT` | `local` | Distinguishes from data-qa in logs |
| `JOBS_ENABLED` | `false` | Prevents duplicate job execution with K8s pod |
| `DB_HOST` | `localhost` | Tunneled via SSM (port 13306) |
| `DB_PORT` | `13306` | SSM tunnel → IRIS primary |
| `DB_PORT_READ_ONLY` | `13307` | SSM tunnel → IRIS read-only |
| `QS_DB_HOST` | `localhost` | Tunneled via SSM (port 13308) |
| `QS_DB_PORT` | `13308` | SSM tunnel → QS MySQL |
| `COGNITO_SSO_REDIRECT_URI` | `http://localhost:3000/login/sso-callback` | SSO redirects to local frontend |

> **Safety:** `JOBS_ENABLED=false` ensures no cron jobs run locally. The K8s pod continues running normally.

### SSM Tunnel EC2 Instance

| Field | Value |
|-------|-------|
| Instance ID | `i-097a5b6d0bf443137` |
| Name | `unified-qual-ssm-tunnel` |
| Type | `t3.micro` (Amazon Linux 2023) |
| VPC | `vpc-7ae31207` (us-east-1, same as databases) |
| Security Group | `sg-03a01fb2e53f47724` (egress: 3306 VPC + 443 internet) |
| IAM Profile | `EC2-SSM-InstanceProfile` |

## Build & Deploy

```bash
make build          # Build binary
make docker-build   # Docker build
make ecr-login
make docker-push TAG=$(git rev-parse --short HEAD)
```
ArgoCD syncs from gitops repo automatically.

## Code Formatting

Go code is formatted with `gofmt` and `goimports`. CI enforces formatting — unformatted code will fail the pipeline.

### Setup

```bash
# Install goimports (one-time)
go install golang.org/x/tools/cmd/goimports@latest
```

### Usage

```bash
make fmt           # Format all Go files with gofmt
make imports       # Format + sort imports (groups: stdlib → third-party → local)
make fmt-check     # Dry-run check — fails if any files need formatting
```

### Configuration

- **`.golangci.yml`** — enables `goimports` linter with local import prefix (`github.com/InCrowd/unified-qual-api`)
- **CI** — `gofmt -l` gate runs before lint in the `lint-test` job
- Import grouping order: stdlib → third-party → `github.com/InCrowd/unified-qual-api`

> **Tip:** Run `make fmt && make imports` before committing to avoid CI failures.

## Testing

### Unit Tests (29 tests, 3 packages)

Unit tests live alongside source files (Go convention): `handler/shared`, `middleware`, `jobs`.

**Run all unit tests:**
```bash
# Step 1: Run all unit tests with race detection + coverage
go test -race -coverprofile=coverage.out ./...

# Step 2 (optional): View coverage report in browser
go tool cover -html=coverage.out

# Step 3 (optional): Run a specific package
go test -v ./internal/handler/shared/
go test -v ./internal/middleware/
go test -v ./internal/jobs/
```

**Run a single test by name:**
```bash
go test -v -run TestHealth_AllDBsHealthy ./internal/handler/shared/
```

### Integration Tests (16 tests, full HTTP stack)

Integration tests live in `integrationtests/` and exercise the full HTTP stack (router → middleware → handler → mock repos). They use the `integration` build tag so `go test ./...` skips them by default.

**These tests also run in CI** — the `lint-test` job runs integration tests after unit tests.

**Run all integration tests:**
```bash
# Step 1: Run all integration tests
go test -tags=integration -v ./integrationtests/...

# Step 2 (optional): Run specific test
go test -tags=integration -run TestProjects_AdminToken ./integrationtests/...
```

**Test structure:**
```
integrationtests/
  testserver/server.go  — TestServer builder (real Chi router, mock deps, test JWT signing)
  health_test.go        — Health endpoint full-stack tests (3 tests)
  auth_test.go          — Auth validation + SSO config + protected route tests (8 tests)
  project_test.go       — JWT auth → RBAC → project list with mock repos + CORS (5 tests)
```

The `testserver.New()` helper creates a real `httptest.Server` with:
- Real Chi router + all middleware (CORS, RequestID, JWT, RBAC)
- Mock repositories (set expectations per-test with `ts.IrisProjectRepo.On(...)`)
- Test RSA key pair for JWT signing (`ts.AdminToken()`, `ts.ManagerToken()`)
- No real databases or external services

### Run All Tests (Unit + Integration)

```bash
go test -race ./... && go test -tags=integration -race ./integrationtests/...
```

### Mocks

Mocks are auto-generated with [mockery](https://github.com/vektra/mockery) in `internal/unittests/mocks/`. To regenerate after interface changes:

```bash
# Step 1: Install mockery
go install github.com/vektra/mockery/v2@latest

# Step 2: Regenerate a specific mock (example: IRIS ProjectRepository)
mockery --dir=internal/repository/iris --name=ProjectRepository \
  --output=internal/unittests/mocks --outpkg=mocks --with-expecter \
  --structname=MockIrisProjectRepository --filename=mock_iris_projectrepository.go
```

## Security Scanning (Snyk)

Snyk runs automatically in CI (`security` job, parallel with `lint-test`):
- **Dependency scan:** `snyk test --all-projects --severity-threshold=high`
- **SAST:** `snyk code test`
- **Monitor:** `snyk monitor` (uploads snapshot to Snyk dashboard on push)

Currently **non-blocking** (`continue-on-error: true`). Remove to enforce after baseline is clean.

`SNYK_TOKEN` is stored as a GitHub Actions secret. To run locally:

```bash
export SNYK_TOKEN=<your-token>
npx snyk test --all-projects
npx snyk code test
```

## CI/CD Pipeline

The pipeline is defined in `.github/workflows/ci-cd.yml`.

**On `data-qa` branch (full pipeline):**
```
lint-test (tidy → format check → lint → 29 unit tests → 16 integration tests)
    ∥
security (Snyk deps → SAST → monitor)
    ↓
build-push (Docker → ECR)
    ↓
deploy [approval gate] → update gitops tag → ArgoCD syncs to EKS
```

**On `qa` / `staging` / `production` branches (deploy only):**
```
deploy [approval gate] → update gitops tag → ArgoCD syncs to EKS
```

- CI jobs (`lint-test`, `security`, `build-push`) only run on `data-qa`.
- Deploy maps each branch to its gitops values path (`data-qa` → `envs/konovo-dev/`, `qa` → `envs/konovo-qa/`, etc.).
- **Approval gate:** Uses GitHub Environments — configure required reviewers in repo Settings → Environments → `<branch-name>`.

## API Endpoints
All routes under `/v1/` prefix. Health check at `/health`. 228 routes total across 14 sub-routers.
