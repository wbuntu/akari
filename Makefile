PHONY = all
GOOS ?= linux
GOARCH ?= amd64
VERSION ?= latest
COMMIT = $(shell git log --format="%h" -n 1|tr -d '\n')
TIMESTAMP = $(shell date -u "+%Y-%m-%dT%H:%M:%SZ")

build:
	@echo "Compiling source for $(GOOS) $(GOARCH)"
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-s -w -X main.version=$(VERSION)-$(COMMIT)-$(TIMESTAMP)" -o akari main.go

image: build
	@echo "Building image for $(GOOS) $(GOARCH)"
	@docker build -t wbuntu/akari:$(VERSION) .

linux:
	@echo "Compiling source for linux amd64"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=$(VERSION)-$(COMMIT)-$(TIMESTAMP)" -o akari_linux_amd64 main.go

windows:
	@echo "Compiling source for windows amd64"
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=$(VERSION)-$(COMMIT)-$(TIMESTAMP)" -o akari_windows_amd64.exe main.go

darwin:
	@echo "Compiling source for darwin amd64"
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.version=$(VERSION)-$(COMMIT)-$(TIMESTAMP)" -o akari_darwin_amd64 main.go

release: linux windows darwin
	@mv akari_linux_amd64 akari && tar -zcvf akari-linux-amd64-$(VERSION).tar.gz akari && rm akari
	@mv akari_windows_amd64.exe akari.exe && tar -zcvf akari-windows-amd64-$(VERSION).tar.gz akari.exe && rm akari.exe
	@mv akari_darwin_amd64 akari && tar -zcvf akari-darwin-amd64-$(VERSION).tar.gz akari && rm akari


