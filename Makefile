.PHONY: dev build test lint sdk e2e install

dev:
	go run ./cmd/artifact dev

build:
	go build -o bin/artifact ./cmd/artifact

test:
	go test ./... -count=1

lint:
	golangci-lint run ./...

sdk:
	cd sdk && npm install && npm run build

e2e:
	go test ./e2e/... -count=1 -timeout 5m

install:
	./scripts/install.sh

docker:
	docker compose -f deploy/docker-compose.yml up --build

.PHONY: all
all: build sdk test
