.PHONY: generate build run

generate:
	sqlc generate

build:
	go build -o build/api .

run:
	go run .