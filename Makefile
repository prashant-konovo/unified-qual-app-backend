.PHONY: build run test docker-build docker-push clean

APP_NAME := unified-qual-api
ECR_REPO := 631543112504.dkr.ecr.us-east-2.amazonaws.com/unified-qual-api
TAG      ?= latest

build:
	go build -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -race -coverprofile=coverage.out

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

imports:
	goimports -w -local github.com/InCrowd/unified-qual-api .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

docker-build:
	docker build -t $(APP_NAME):$(TAG) .

docker-tag: docker-build
	docker tag $(APP_NAME):$(TAG) $(ECR_REPO):$(TAG)

docker-push: docker-tag
	docker push $(ECR_REPO):$(TAG)

ecr-login:
	aws ecr get-login-password --region us-east-2 | docker login --username AWS --password-stdin 631543112504.dkr.ecr.us-east-2.amazonaws.com

clean:
	rm -rf bin/ coverage.out
