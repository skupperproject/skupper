package main

//go:generate go run github.com/oapi-codegen/oapi-codegen/main/cmd/oapi-codegen --config=.oapi-codegen.cfg ./spec/openapi.yaml
//go:generate go run ./codegen -o ./internal/api/extras_gen.go ./internal/api/types_gen.go
//go:generate go fmt ./internal/api
