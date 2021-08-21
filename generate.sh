#!/bin/bash

set -e

which protoc
which protoc-gen-gogofaster
which gomplate
which easyjson
which goimports

gomplate -f client.template > definitions/client.proto
GOGO=1 gomplate -f client.template > client.proto

go mod vendor
cat client.proto
protoc --proto_path=vendor/:. --gogofaster_out=plugins=grpc:. client.proto
rm client.proto
rm -rf vendor

# compile easy json in separate dir since we are using custom writer here.
mkdir build
cp client.pb.go build/client.pb.go
cp raw.go build/raw.go
cd build
easyjson -all -no_std_marshalers client.pb.go
cd ..
# move compiled to current dir.
cp build/client.pb_easyjson.go ./client.pb_easyjson.go
rm -rf build

# need to replace usage of jwriter.Writer to custom writer.
find . -name 'client.pb_easyjson.go' -print0 | xargs -0 sed -i "" "s/jwriter\.W/w/g"
# need to replace usage of jwriter package constants to local writer constants.
find . -name 'client.pb_easyjson.go' -print0 | xargs -0 sed -i "" "s/jwriter\.N/n/g"
# cleanup formatting.
goimports -w client.pb_easyjson.go
