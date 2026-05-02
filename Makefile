BINARY := jira-thing
SOURCES := $(shell find . -name '*.go' -not -path './.git/*')
GOBIN   := $(shell go env GOPATH)/bin

.PHONY: all build test lint security clean

tools:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
        
all: lint security test clean build

build: $(BINARY)

$(BINARY): $(SOURCES) go.mod go.sum
	go build -o $(BINARY) .

test:
	go test -cover ./...

lint:
	go vet ./...
	gofmt -w .
	$(GOBIN)/staticcheck ./...

security:
	$(GOBIN)/gosec -exclude-dir=.agents ./...
	$(GOBIN)/govulncheck ./...

clean:
	rm -f $(BINARY)
