build:
	@go build -o bin/botservice

run: build
	@./bin/botservice

test:
	@go test -v ./...
