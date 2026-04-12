.PHONY: buf-generate generate build test

buf-generate:
	buf dep update
	buf generate

generate: buf-generate
	go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.1 object:headerFile=hack/boilerplate.go.txt paths=./apis/...
	go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.1 crd:crdVersions=v1 paths=./apis/... output:crd:artifacts:config=package/crds

build:
	go build -o bin/provider ./cmd/provider

test:
	go test ./...
