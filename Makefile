BINARY := jira-thing

.PHONY: build test clean vet tidy

build:
	go build -o $(BINARY) .

test:
	go test ./...

clean:
	rm -f $(BINARY)

vet:
	go vet ./...

tidy:
	go mod tidy
