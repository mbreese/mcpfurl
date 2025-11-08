all: bin/mcpfurl.linux

SOURCES := $(shell find . -name '*.go')

bin/mcpfurl.linux: go.mod go.sum $(SOURCES)
	CGO_ENABLED=1 GOOS=linux go build -o bin/mcpfurl.linux main.go

bin/mcpfurl.linux_musl_amd64: go.mod go.sum $(SOURCES)
	CC=x86_64-linux-musl-gcc \
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags='-extldflags "-static"' -o bin/mcpfurl.linux_musl_amd64 main.go

bin/mcpfurl.linux_musl_arm64: go.mod go.sum $(SOURCES)
	CC=aarch64-linux-musl-gcc \
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -ldflags='-extldflags "-static"' -o bin/mcpfurl.linux_musl_arm64 main.go

bin/mcpfurl.macos_amd64: go.mod go.sum $(SOURCES)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/mcpfurl.macos_amd64 main.go

bin/mcpfurl.macos_arm64: go.mod go.sum $(SOURCES)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o bin/mcpfurl.macos_arm64 main.go

bin/mcpfurl.macos: bin/mcpfurl.macos_arm64 bin/mcpfurl.macos_amd64
	lipo -create -output bin/mcpfurl.macos bin/mcpfurl.macos_amd64 bin/mcpfurl.macos_arm64

clean:
	rm bin/*

run:
	CGO_ENABLED=1 go run main.go

.PHONY: run clean
