all:
	go vet ./...
	go test -race ./...
	CGO_ENABLED=0 go build -o steth ./cmd/steth

run: all
	./steth

.PHONY: all run
