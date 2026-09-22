GRAMMAR_TAGS := grammar_subset,grammar_subset_go,grammar_subset_javascript,grammar_subset_typescript,grammar_subset_tsx,grammar_subset_python,grammar_subset_java,grammar_subset_rust,grammar_subset_c_sharp,grammar_subset_php,grammar_subset_ruby

.PHONY: build test smoke index web web-install

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -tags "$(GRAMMAR_TAGS)" -o bin/whichrepo ./cmd/whichrepo

test:
	CGO_ENABLED=0 go test -tags "$(GRAMMAR_TAGS)" ./...
	CGO_ENABLED=0 go vet -tags "$(GRAMMAR_TAGS)" ./...
	cd web && npm run build
	cd web && npm run test:sites

smoke: build
	./scripts/smoke-binary.sh ./bin/whichrepo

index: build
	./bin/whichrepo index --workspace .. --config ../.whichrepo.local.yaml

web-install:
	cd web && npm install --no-audit --no-fund

web:
	cd web && npm run build
	rm -rf internal/dashboard/static
	mkdir -p internal/dashboard/static
	cp -R web/dist/client/. internal/dashboard/static/
