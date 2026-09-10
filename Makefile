.PHONY: test-crosswalk test-studio test-studio-backend test-studio-local build-crosswalk build-studio build-studio-backend dev-studio-local smoke-studio-local

test-crosswalk:
	cd crosswalk && go test ./...

test-studio:
	cd studio && ./node_modules/.bin/tsc --noEmit
	cd studio && node --test features/topology/model.test.mjs
	cd studio && node --test features/topology/api-adapter.test.mjs
	cd studio && node --test schemas/contracts.test.mjs
	cd studio && node --test lib/api/client.test.mjs
	cd studio && node --test features/studio/validation-state.test.mjs

test-studio-backend:
	cd studio/backend && go test ./...

test-studio-local: test-studio-backend test-studio

build-crosswalk:
	cd crosswalk && go build -o bin/deploy ./cmd/deploy

build-studio:
	cd studio && npm run build

build-studio-backend:
	cd studio/backend && go build -o bin/studio-api ./cmd/studio-api

dev-studio-local:
	cd studio && ./scripts/dev-local.sh

smoke-studio-local:
	cd studio && ./scripts/smoke-local.sh
