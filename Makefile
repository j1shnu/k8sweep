.PHONY: build run test coverage lint clean

BINARY := dist/k8sweep

build:
	go build -o $(BINARY) .

run:
	go run .

test:
	go test ./... -race -coverprofile=coverage.out

coverage: test
	go tool cover -func=coverage.out

lint:
	golangci-lint run

clean:
	rm -rf dist/ coverage.out
