# cfprotobuf

> **For the Centrifugal ecosystem only.** This is not a general-purpose
> package. It is built for the needs of Centrifugal projects (Centrifugo,
> centrifuge, their SDKs and tools), supports what their schemas use and
> nothing more, and changes as they need it to: its API, the code it
> generates and its behaviour may change in any release, without notice
> and without a major version. Please do not depend on it outside of the
> Centrifugal ecosystem.

A code generator for Protobuf marshaling and unmarshaling of the Go structs
`protoc-gen-go` produces, and the small runtime package the generated code is
built on.

It was written to replace
[vtprotobuf](https://github.com/planetscale/vtprotobuf) in
`github.com/centrifugal/protocol`. vtprotobuf is not a part of
`google.golang.org/protobuf`, and the code it generates is what reads bytes
from the network, so it is code worth having control over. The generated
methods do the same and produce the same bytes. They are named after this
package, `MarshalCF` where vtprotobuf has `MarshalVT`; generated with
`-suffix VT` they are a drop-in replacement.

The messages stay regular Protobuf messages: the structs still come from
`protoc-gen-go`, and everything in `google.golang.org/protobuf` (reflection,
`protojson`, gRPC) works with them as before. cfprotobuf only provides the
fast path which does not go through reflection.

The package depends on the standard library only.

## Usage

```
go run github.com/centrifugal/protocol/cfprotobuf/cmd/cfprotobuf \
    -out client.pb_cfprotobuf.go client.pb.go raw.go
```

All files given are parsed, which is how the generator learns about types the
structs refer to (`raw.go` declares the `Raw` type used for payloads). Code is
generated for every struct which has fields with a `protobuf` tag.

| Flag | Meaning |
| --- | --- |
| `-out` | Output file. Defaults to the first input file with the `_cfprotobuf.go` suffix. |
| `-types` | Comma-separated structs to generate code for. All messages of the input files by default. |
| `-suffix` | What the names of generated methods end with, `CF` by default. `VT` gives the names vtprotobuf uses. |
| `-drop-unknown` | Do not keep the fields a message does not have when decoding. |
| `-runtime` | Import path of this package, for when it is moved or vendored. |

For every message the generator writes:

```go
// MarshalCF returns the Protobuf encoding of m.
func (m *T) MarshalCF() ([]byte, error)

// MarshalToCF writes the encoding to the beginning of b, which must have
// room for SizeCF bytes.
func (m *T) MarshalToCF(b []byte) (int, error)

// MarshalToSizedBufferCF writes the encoding to the end of b.
func (m *T) MarshalToSizedBufferCF(b []byte) (int, error)

// SizeCF returns the size of the encoding.
func (m *T) SizeCF() int

// UnmarshalCF decodes the encoding of a message from b into m.
func (m *T) UnmarshalCF(b []byte) error
```

The generator reads field numbers and encodings from the struct tags
`protoc-gen-go` writes, so it needs no `protoc` plugin and no descriptor.

## What is supported

The subset of proto3 which the Centrifugal schemas use:

- `string`, `bytes` (as `[]byte` or a named type based on it), `bool`;
- `int32`, `int64`, `uint32`, `uint64` and named types based on them;
- `sint64`, `float`, `double`;
- nested messages, by pointer;
- repeated messages and repeated strings;
- maps with string keys and values of any of the above.

Anything else is an error at generation time which names the field: fields
which are not proto3, `oneof`, enums (as map values too), repeated numbers (packed or not), `sint32`, `fixed32`/`fixed64` and
`sfixed` integers, maps with other keys, proto2 `required` fields. A field
silently left out would be data lost on the wire, so there is no partial
support.

## Encoding

The output is byte for byte what vtprotobuf produces, which is also what
`google.golang.org/protobuf` produces with deterministic marshaling (up to
the order of map entries, which is random for both generators):

- fields are written in the order of their numbers;
- a field which holds the zero value is left out, as proto3 has it;
- a nil element of a repeated field and a nil map value are written as empty
  messages;
- fields a message did not know when it was decoded are written back after
  the known ones.

## Decoding

Decoding follows `google.golang.org/protobuf`, the reference implementation.
Generated code is tested against it on random, damaged and fuzzed input: the
two must agree on whether an input is acceptable and on the message it holds.
The one exception is the nesting limit for recursive types, which is lower
here.

- A field a message does not have is skipped and kept with the message (if
  the struct has the `unknownFields` field `protoc-gen-go` adds). With the
  `-drop-unknown` option it is skipped and forgotten, which is what
  `protocol` is generated with: a server has no use for fields it does not
  know, and nothing to pass them on to.
- A field a message has which came with another wire type than its type has
  is treated the same way.
- Decoding into a message which has something in it merges: nested messages
  are merged, repeated fields are appended to, everything else is replaced.
- Everything decoded is a copy, the input may be reused afterwards.

Errors:

- the input ends in the middle of a value, or a length points past its end;
- a varint which does not fit 64 bits;
- a field number outside of 1..2^29-1 (1..2^31-1 for the fields of a group
  which is skipped, as `google.golang.org/protobuf` has it);
- a group which is not closed, or closed with another number;
- a string field which is not valid UTF-8 (`bytes` fields may hold anything);
- messages of a recursive type nested deeper than `cfprotobuf.MaxDepth`, or
  more than 10000 groups nested in a field which is skipped.

## Differences with vtprotobuf

On valid input there is none. On input which is not valid vtprotobuf departs
from the specification in several ways, and cfprotobuf does not follow it
there:

| Input | vtprotobuf | cfprotobuf and `google.golang.org/protobuf` |
| --- | --- | --- |
| A string which is not valid UTF-8 | accepted | error |
| Messages of a recursive type nested deeper than 1024 levels | recurses as deep as the input goes | error (`google.golang.org/protobuf` allows 10000 levels) |
| A varint of ten bytes whose value does not fit 64 bits | accepted, upper bits dropped | error |
| A field number above 2^29-1 | accepted, and mistaken for a known field if its low 32 bits match one | error |
| A group closed with another field number | accepted | error |
| A known field with another wire type | error | kept as an unknown field |
| The same inside of a map entry | read as if it had the expected wire type | skipped |
| A map entry without the value, when the value is a message | nil in the map | an empty message |
| A map entry with a message value more than once | the last one | merged |
| More than 10000 groups nested in a field which is skipped | accepted | error |
| A length inside of a map entry which goes past the entry | accepted if it stays within the message | error |
| The tag of an unknown field not in its shortest form | kept as it came | kept in the shortest form |

## Safety

Decoders are meant for untrusted input:

- they never read outside of the input, never modify it and never panic,
  which is fuzzed;
- skipping unknown fields, groups included, does not recurse;
- generated decoders recurse only as deep as the types nest. For types which
  can contain themselves the depth comes from the input, so it is limited to
  `cfprotobuf.MaxDepth`, 1024 levels;
- no `unsafe`, in the package or in generated code;
- the package and the generated code pass the linter set of the repository
  (gosec, staticcheck, gocritic, `go vet` with every analyser on), and the
  tests also run where `int` is 32 bits and where the byte order is
  big-endian.

## Performance

Speed was not the reason to write this: vtprotobuf generates good code, and
cfprotobuf generates much the same. See [BENCHMARKS.md](BENCHMARKS.md) for
the comparison, and [../internal/cfprotobufcmp](../internal/cfprotobufcmp)
for the benchmarks and for the tests which compare the two.

A few things the generated code does to keep up while doing more:

- A marshaler fills its buffer from the end: a nested message is written
  first and its length after it, so no size is computed twice.
- Decoding switches on the field number, which the compiler turns into a jump
  table, and decodes varints of one byte (most tags, lengths and numbers) in
  place.
- Whether a string is ASCII, and so valid, is checked a word at a time.
- Iterating over a map is skipped when the map is empty, which is not free in
  Go even then.
