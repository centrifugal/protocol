all: proto

install:
	GO111MODULE=off go get -u github.com/gogo/protobuf/protoc-gen-gogofaster
	GO111MODULE=off go get github.com/hairyhenderson/gomplate
	GO111MODULE=off go get -u github.com/mailru/easyjson/...

proto:
	bash generate.sh

test:
	go test -v
