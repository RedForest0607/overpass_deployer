.PHONY: generate-studio-api check-studio-api-generated test-crosswalk test-compose-fixture smoke-compose-real smoke-compose-podman test-studio test-studio-backend test-studio-backend-race test-studio-operational test-studio-e2e test-studio-browser-e2e test-studio-local build-crosswalk build-studio build-studio-backend build-studio-admin build-studio-release dev-studio-local smoke-studio-local

generate-studio-api:
	cd studio && npm run generate:api

check-studio-api-generated:
	cd studio && npm run check:api-generated

test-crosswalk:
	cd crosswalk && go test ./...

test-compose-fixture:
	node --test crosswalk/tests/compose/podman-smoke.test.mjs
	cd crosswalk && go test ./internal/compose -run TestComposeApplySSHFixtureLifecycle -v

smoke-compose-real:
	cd crosswalk && bash ./tests/compose/run-real-smoke.sh

smoke-compose-podman:
	cd crosswalk && bash ./tests/compose/run-podman-smoke.sh

test-studio: check-studio-api-generated
	cd studio && ./node_modules/.bin/tsc --noEmit
	cd studio && node --test features/topology/model.test.mjs
	cd studio && node --test features/topology/api-adapter.test.mjs
	cd studio && node --test features/topology/communication-edge-layout.test.mjs
	cd studio && node --test features/topology/topology-import.test.mjs
	cd studio && node --test features/topology/usePointerDrag.test.mjs
	cd studio && node --test schemas/contracts.test.mjs
	cd studio && node --test lib/api/client.test.mjs
	cd studio && node --test lib/api/delivery.test.mjs
	cd studio && node --test features/artifacts/artifact-mapping.test.mjs
	cd studio && node --test features/artifacts/artifact-text.test.mjs
	cd studio && node --test features/generation/generation-state.test.mjs
	cd studio && node --test features/generation/package-contract.test.mjs
	cd studio && node --test features/studio/validation-state.test.mjs
	cd studio && node --test features/studio/unsaved-navigation.test.mjs

test-studio-backend:
	cd studio/backend && go test ./...

test-studio-backend-race:
	cd studio/backend && go test -race ./...

test-studio-operational: test-studio-backend-race
	cd studio/backend && go vet ./...

test-studio-e2e:
	cd studio && node scripts/e2e-studio-crosswalk.mjs

test-studio-browser-e2e:
	cd studio && npm run test:e2e:browser

test-studio-local: test-studio-backend test-studio test-studio-e2e test-studio-browser-e2e

build-crosswalk:
	cd crosswalk && go build -o bin/deploy ./cmd/deploy

build-studio:
	cd studio && npm run build

build-studio-backend:
	cd studio/backend && go build -o bin/studio-api ./cmd/studio-api

build-studio-admin:
	cd studio/backend && go build -o bin/studio-admin ./cmd/studio-admin

build-studio-release: build-studio-backend build-studio-admin build-studio

dev-studio-local:
	cd studio && ./scripts/dev-local.sh

smoke-studio-local:
	cd studio && ./scripts/smoke-local.sh
