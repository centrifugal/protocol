#!/bin/bash
#
# Regenerates Go code from client.proto.
#
# The result is three generated files, all of them committed to the repo:
#
#   client.pb.go           - protoc-gen-go structs, with []byte fields replaced by Raw.
#   client_vtproto.pb.go   - allocation-friendly Protobuf marshal/unmarshal/size methods.
#   client.pb_easyjson.go  - easyjson marshalers, rewired to the writer from encode_writer.go.
#
# Required tools:
#
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@latest
#   go install github.com/fatih/gomodifytags@v1.13.0
#   go install github.com/FZambia/gomodifytype@latest
#   go install github.com/mailru/easyjson/easyjson@v0.7.7
#   go install golang.org/x/tools/cmd/goimports@latest
#
# The easyjson binary version must match the github.com/mailru/easyjson version
# in go.mod - keep both pinned when upgrading.

set -euo pipefail

cd "$(dirname "$0")"

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: $1 not found in PATH, see the comment on top of generate.sh" >&2
    exit 1
  fi
  echo "using $1: $(command -v "$1")"
}

require protoc
require protoc-gen-go
require protoc-gen-go-vtproto
require gomodifytype
require gomodifytags
require easyjson
require goimports

echo "generating Protobuf code..."
protoc --go_out=. --plugin protoc-gen-go="$(command -v protoc-gen-go)" --go-vtproto_out=. \
  --plugin protoc-gen-go-vtproto="$(command -v protoc-gen-go-vtproto)" \
  --go-vtproto_opt=features=marshal+unmarshal+size \
  client.proto

# protoc writes into a directory tree matching go_package, move results to the repo root.
cp github.com/centrifugal/protocol/client.pb.go client.pb.go
cp github.com/centrifugal/protocol/client_vtproto.pb.go client_vtproto.pb.go
rm -rf github.com

echo "replacing []byte fields with Raw type..."
gomodifytype -file client.pb.go -all -w -from "[]byte" -to "Raw"

echo "replacing tags of structs for JSON backwards compatibility..."
gomodifytags -file client.pb.go -field User -struct ClientInfo -all -w -remove-options json=omitempty >/dev/null
gomodifytags -file client.pb.go -field Client -struct ClientInfo -all -w -remove-options json=omitempty >/dev/null
gomodifytags -file client.pb.go -field Presence -struct PresenceResult -all -w -remove-options json=omitempty >/dev/null
gomodifytags -file client.pb.go -field NumClients -struct PresenceStatsResult -all -w -remove-options json=omitempty >/dev/null
gomodifytags -file client.pb.go -field NumUsers -struct PresenceStatsResult -all -w -remove-options json=omitempty >/dev/null
gomodifytags -file client.pb.go -field Offset -struct HistoryResult -all -w -remove-options json=omitempty >/dev/null
gomodifytags -file client.pb.go -field Epoch -struct HistoryResult -all -w -remove-options json=omitempty >/dev/null
gomodifytags -file client.pb.go -field Publications -struct HistoryResult -all -w -remove-options json=omitempty >/dev/null

echo "generating easyjson code..."
# Compile easyjson in a separate dir since we are using a custom writer here.
rm -rf build
mkdir build
cp client.pb.go build/client.pb.go
cp raw.go build/raw.go
(cd build && easyjson -all -no_std_marshalers client.pb.go)
# Move compiled file to the current dir.
cp build/client.pb_easyjson.go ./client.pb_easyjson.go
rm -rf build

# Replace usage of jwriter.Writer with the custom writer from encode_writer.go and
# usage of jwriter package constants with local writer constants. Note: not using
# `sed -i` here since its syntax differs between GNU and BSD sed.
sed -e 's/jwriter\.W/w/g' -e 's/jwriter\.N/n/g' client.pb_easyjson.go > client.pb_easyjson.go.tmp
mv client.pb_easyjson.go.tmp client.pb_easyjson.go
# Cleanup formatting.
goimports -w client.pb_easyjson.go

# Copy to definitions folder for docs link backwards compatibility.
cp client.proto definitions/client.proto

echo "done"
