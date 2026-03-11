.PHONY: build test clean run

BINARY_NAME=red-queen
MAIN_FILE=cmd/red-queen/main.go

build:
	go build -o $(BINARY_NAME) $(MAIN_FILE)

test:
	go test ./...

integration-test: build
	go test -v -tags=integration ./...

clean:
	rm -f $(BINARY_NAME)

run: build
	./$(BINARY_NAME)
