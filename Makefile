PHONY = all
GOOS ?= linux
GOARCH ?= amd64
VERSION ?= latest
COMMIT = $(shell git log --format="%h" -n 1|tr -d '\n')
TIMESTAMP = $(shell date -u "+%Y-%m-%dT%H:%M:%SZ")

build:
	@echo "Compiling source for $(GOOS) $(GOARCH)"
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-s -w -X main.version=$(VERSION)-$(COMMIT)-$(TIMESTAMP)" -o akari main.go