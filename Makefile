all: proto

proto:
	bash generate.sh

test:
	go test -v
