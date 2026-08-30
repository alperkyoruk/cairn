# Cairn builds in two steps: the Vue app compiles into web/dist, then the Go
# build embeds that directory into the binary. `go build` alone produces a
# working API with no web interface, which is why `build` is the default.

BINARY := cairn

.PHONY: build web go test clean dev

build: web go

web:
	cd web && npm install --silent && npm run build

go:
	go build -o $(BINARY) ./cmd/cairn

test:
	go test ./...

# Run the server and the Vue dev server side by side. The dev server proxies
# /api to the Go process, so cookies stay same-origin.
dev:
	@echo "run these in two terminals:"
	@echo "  go run ./cmd/cairn"
	@echo "  cd web && npm run dev"

clean:
	rm -rf $(BINARY) web/dist/* 
	touch web/dist/.gitkeep
