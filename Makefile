.PHONY: all build run test coverage lint tidy clean

BINARY_NAME=niceboy

all: build

build: tidy
	go build -o $(BINARY_NAME) cmd/niceboy/main.go

run: build
	./$(BINARY_NAME)

test:
	go test -v ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm coverage.out

lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, running go vet instead..."; \
		go vet ./...; \
	fi

tidy:
	go mod tidy

install-hooks:
	@echo "Installing git hooks..."
	@mkdir -p .git/hooks
	@ln -sf ../../scripts/git-hooks/pre-commit.sh .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Hooks installed successfully!"

clean:
	rm -f $(BINARY_NAME)
	rm -f *.log
	rm -f coverage.out
