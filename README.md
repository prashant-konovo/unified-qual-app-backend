# Unified Qual API — Go + Chi Backend

Dummy backend that returns stub responses for all Unified Qual frontend API calls.  
Deployed to EKS (data-qa) via ArgoCD.

## Tech Stack
- **Go 1.22** + **Chi v5** router
- Docker multi-stage build → ECR → EKS

## Local Development
```bash
make run     # Run locally on :8080
make build   # Build binary
make docker-build  # Docker build
```

## API Endpoints
All routes under `/v1/` prefix. Health check at `/health`.

## Deployment
```bash
make ecr-login
make docker-push TAG=$(git rev-parse --short HEAD)
```
ArgoCD syncs from gitops repo automatically.
