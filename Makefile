BINARY=clawdaemon
VERSION=v0.1.0-alpha
BUILD_DIR=build

.PHONY: build run test fmt vet clean deps release

# Local build for current OS/arch — NO CGO needed
build:
	go build -ldflags="-X main.Version=$(VERSION)" -o $(BINARY) .

# Windows build produces clawdaemon.exe
build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-X main.Version=$(VERSION)" -o $(BINARY).exe .

run: build
	./$(BINARY)

test:
	go test ./... -v -race

fmt:
	gofmt -w .

vet:
	go vet ./...

lint: fmt vet

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf $(BUILD_DIR)

deps:
	go mod download
	go mod tidy

# Cross-compile all platforms from any machine — no CGO needed!
release:
	mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64  go build -o $(BUILD_DIR)/$(BINARY)-linux-amd64  .
	GOOS=linux   GOARCH=arm64  go build -o $(BUILD_DIR)/$(BINARY)-linux-arm64  .
	GOOS=darwin  GOARCH=amd64  go build -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64  go build -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64  go build -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe .
	@echo ""
	@echo "✅ Built for all platforms:"
	@ls -lh $(BUILD_DIR)/

docker:
	docker build -f docker/Dockerfile -t clawdaemon:$(VERSION) .

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down
