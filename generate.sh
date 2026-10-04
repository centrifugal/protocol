#!/bin/bash
#
# Regenerates Go code from client.proto.
#
# The result is three generated files, all of them committed to the repo:
#
#   client.pb.go             - protoc-gen-go structs, with []byte fields replaced by Raw.
#   client.pb_cfprotobuf.go  - Protobuf marshal/unmarshal/size methods, see cfprotobuf.
#   client.pb_cfjson.go      - JSON encoders and decoders, see cfjson.
#
# Required tools:
#
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install github.com/fatih/gomodifytags@v1.13.0
#   go install github.com/FZambia/gomodifytype@latest
#
# The generators of the JSON and of the Protobuf code live in this repo
# (cfjson/cmd/cfjson and cfprotobuf/cmd/cfprotobuf) and are run with `go run`,
# so they need no installation and cannot go out of sync.

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
require gomodifytype
require gomodifytags

echo "generating Protobuf structs..."
protoc --go_out=. --plugin protoc-gen-go="$(command -v protoc-gen-go)" client.proto

# protoc writes into a directory tree matching go_package, move results to the repo root.
cp github.com/centrifugal/protocol/client.pb.go client.pb.go
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

echo "generating Protobuf code..."
# raw.go is there for the declaration of Raw, which the structs refer to.
# Fields a message does not have are not kept: nothing here passes on what it
# decoded without knowing what it is.
go run ./cfprotobuf/cmd/cfprotobuf -drop-unknown -out client.pb_cfprotobuf.go client.pb.go raw.go

echo "generating JSON code..."
# Raw holds an already encoded JSON value.
# validRaw is what the encoders of replies and pushes check payloads with.
go run ./cfjson/cmd/cfjson -raw Raw -valid-raw validRaw -out client.pb_cfjson.go client.pb.go

# Copy to definitions folder for docs link backwards compatibility.
cp client.proto definitions/client.proto

echo "done"
