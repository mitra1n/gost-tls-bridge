BINARY := gost-tls-bridge
VERSION := 0.1.0
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build build-windows build-linux dist release fmt vet test clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .

dist: build-windows build-linux

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf bin dist

# Reproducible multi-OS release artifacts under dist/.
PLATFORMS := windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
release:
	@rm -rf dist && mkdir -p dist
	@for pl in $(PLATFORMS); do \
	  os=$${pl%/*}; arch=$${pl#*/}; \
	  name=$(BINARY)-$(VERSION)-$$os-$$arch; \
	  ext=; if [ "$$os" = "windows" ]; then ext=.exe; fi; \
	  echo "building $$name"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$name/$(BINARY)$$ext . || exit 1; \
	  cp README.md LICENSE dist/$$name/; \
	  if [ "$$os" = "windows" ]; then cp bridge.example.windows.conf dist/$$name/bridge.example.conf; \
	  else cp bridge.example.unix.conf dist/$$name/bridge.example.conf; fi; \
	  if [ "$$os" = "windows" ]; then (cd dist && zip -qr $$name.zip $$name); \
	  else (cd dist && tar -czf $$name.tar.gz $$name); fi; \
	  rm -rf dist/$$name; \
	done
	@cd dist && (sha256sum *.zip *.tar.gz 2>/dev/null > SHA256SUMS.txt || shasum -a 256 *.zip *.tar.gz > SHA256SUMS.txt)
	@echo "--- dist ---" && ls -1 dist
