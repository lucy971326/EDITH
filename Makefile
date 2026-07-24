.PHONY: help backend-run web-dev backend-test web-check web-lint check backend-build web-build build

help:
	@echo "EDITH commands:"
	@echo "  make backend-run  - start the Go backend"
	@echo "  make web-dev      - start the Next.js development server"
	@echo "  make check        - run backend tests and TypeScript checks"
	@echo "  make build        - build backend and web"

backend-run:
	cd backend && go run .

web-dev:
	cd web && npm run dev

backend-test:
	cd backend && go test ./...

web-check:
	cd web && npx tsc --noEmit

web-lint:
	cd web && npm run lint

check: backend-test web-check

backend-build:
	cd backend && go build .

web-build:
	cd web && npm run build

build: backend-build web-build
