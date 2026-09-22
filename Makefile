.PHONY: build test index web web-install

build:
	mkdir -p bin
	go build -o bin/whichrepo ./cmd/whichrepo

test:
	go test ./...
	go vet ./...
	cd web && npm run build
	cd web && npm run test:sites

index: build
	./bin/whichrepo index --workspace .. --config ../.whichrepo.local.yaml

web-install:
	cd web && npm install --no-audit --no-fund

web:
	cd web && npm run build
	mkdir -p internal/dashboard/static
	cp -R web/dist/client/. internal/dashboard/static/
