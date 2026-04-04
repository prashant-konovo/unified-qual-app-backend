# Unified Qual API — Go + Chi Backend

Backend API for the Unified Qualitative Research platform.  
Go 1.24 + Chi v5. Deployed to EKS (data-qa) via ArgoCD.

## Tech Stack
- **Go 1.24** + **Chi v5** router
- **robfig/cron/v3** — 9 scheduled jobs
- MySQL (IRIS + QS-Tool) + DocumentDB
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
| Security Group | `sg-03a01fb2e53f47724` (egress: 3306/27017 VPC + 443 internet) |
| IAM Profile | `EC2-SSM-InstanceProfile` |

## Build & Deploy

```bash
make build          # Build binary
make docker-build   # Docker build
make ecr-login
make docker-push TAG=$(git rev-parse --short HEAD)
```
ArgoCD syncs from gitops repo automatically.

## Testing

```bash
make test               # Run all tests with race detection + coverage
go test -v ./...        # Verbose output
go test ./internal/handler/shared/  # Run specific package tests
```

Mocks are auto-generated with [mockery](https://github.com/vektra/mockery) in `internal/testutil/mocks/`. To regenerate after interface changes:

```bash
go install github.com/vektra/mockery/v2@latest
mockery --dir=internal/repository/iris --name=ProjectRepository --output=internal/testutil/mocks --outpkg=mocks --with-expecter --structname=MockIrisProjectRepository --filename=mock_iris_projectrepository.go
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

## API Endpoints
All routes under `/v1/` prefix. Health check at `/health`. 228 routes total across 14 sub-routers.
