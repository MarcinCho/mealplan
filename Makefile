.PHONY: web build test run

web:
	cd web && npm ci && npm run build

build: web
	CGO_ENABLED=0 go build -o bin/mealplan ./cmd/mealplan

test:
	go test ./...
	cd web && npm test

run: build
	MEALPLAN_COOKIE_SECURE=false ./bin/mealplan
