GOPATH:=$(shell go env GOPATH)

.PHONY: proto test docker


proto:
	protoc --proto_path=${GOPATH}/src:. --micro_out=. --go_out=. proto/chat.proto