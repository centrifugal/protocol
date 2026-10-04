# Follow-ups in the Centrifugo ecosystem

Things noticed while replacing easyjson and segmentio in
`github.com/centrifugal/protocol` which would make JSON encoding and decoding
cleaner or cheaper, but which are outside of what a drop-in replacement may
change. None of them is implemented.

Numbers quoted in this document are from quick measurements on an Apple M4
(arm64) unless said otherwise; the full tables in
[BENCHMARKS.md](BENCHMARKS.md) are from a Linux amd64 machine. Claims about
centrifuge call sites come from a quick search, not from reading the code
paths end to end: verify them before acting.

## Efficiency

### 1. Validate payloads once, where they enter

`JSONReplyEncoder.Encode` and `JSONPushEncoder.Encode` validate the whole
encoded message on every call, because `Raw` payloads come from the
application and may be anything.

For a publication with a 256 byte payload validation is about half of the
cost of encoding (roughly 68 ns out of 130 ns).

A payload has two ways in:

- a client publish, RPC or connect command: the decoder has already validated
  the raw value by the time the command exists;
- the server API: it could validate once when the publication is accepted.

If `Raw` is valid by construction, the encoders have nothing left to check:
everything around the payload is written by generated code. The check could
stay available as an explicit function for code which builds messages from
unchecked bytes.

This changes behaviour: an invalid payload is rejected where it enters
instead of where it leaves.

Decided: this is the way to go, and it is work for centrifuge, not for this
package. It also covers the encoders which do not validate at all today
(`EncodePublication` and the other ones for parts of a message write a
payload as it is given).

### 2. An append-style encoder API

A message currently travels like this:

1. generated code writes it into a pooled scratch buffer;
2. `encodeJSON` copies it into a new slice of the exact size (one allocation);
3. `DataEncoder.Encode` copies that slice into the frame buffer;
4. `DataEncoder.Finish` copies the frame.

Every message type now has a generated `AppendJSON(b []byte) []byte` method,
so a caller which owns the frame buffer can already have a message written
straight into it: one allocation and two copies fewer per message. What is
missing is the rest of what the encoders do around it, the delimiter between
messages and (until item 1 is done) validation, as an API along the lines of

```go
AppendReply(frame []byte, reply *Reply) ([]byte, error)
```

The existing interfaces (`ReplyEncoder`, `PushEncoder`, `DataEncoder`) would
stay for compatibility.

The `reuse ...[]byte` arguments of `PushEncoder` are a partial version of the
same idea, and could be implemented on top of it.

### 2a. Vectorized validation on amd64

On arm64 cfjson validated faster than segmentio for every message in an
earlier measurement. On amd64 it is about level for typical messages and
clearly behind when a payload is one long string (see
[BENCHMARKS.md](BENCHMARKS.md): 65% slower for a 4 KiB string): segmentio
checks "printable ASCII only" with AVX2 assembly, 32 bytes per instruction,
where pure Go does 8 bytes per word.

Three ways to close the gap, in the order I would consider them. The first
one is what was chosen, the other two are not planned:

- item 1 above, which removes most validation instead of speeding it up;
- a build-tagged file using Go's `simd/archsimd` package for that one loop.
  It only takes effect for binaries built with `GOEXPERIMENT=simd`, which
  Centrifugo could do, and the API is not stable yet as of Go 1.27;
- assembly of our own. It would be some 20 lines for SSE2, which every amd64
  CPU has, but assembly is exactly the kind of code that is hard to review
  and that the analysers used here cannot check.

What keeps the door open for it: everything which looks at a run of bytes
is a function of the runtime package (`printableASCII`, `scanQuote`,
`scanControl`, the loops of `AppendString`; `ValidString` in cfprotobuf), and
generated code only calls them. A vectorized version of one of them is a
change to one function, behind a build tag, with the word at a time code as
the fallback and as what it is tested against; nothing has to be generated
again. Optimizations are held to that: no scanning loops in generated code,
and nothing which makes the result of a scan depend on how it was done.

### 3. Allocate a Command together with its request

Decoding a command is dominated by allocation: the `Command`, the request
struct it points to (`PublishRequest`, `SubscribeRequest`, ...), and the copy
of the raw payload. The first two could come from a single allocation:

```go
type publishCommand struct {
	Command
	request PublishRequest
}
```

It is still a new `Command` for every decode, so handlers may keep it for as
long as they like, which centrifuge relies on (callbacks may run after
`HandleCommand` returns). The cost is that a `Command` and its request are
then freed together.

This would be written by hand in `protocol`, or become a generator option for
"allocate this pointer field inline".

## Cleanliness

### 4. One generator for all schemas

Centrifugo PRO has two more copies of `encode_writer.go` and of the
"compile easyjson in a build directory, then `sed` the writer type" script,
for `internal/apiproto` and `internal/proxyproto`. It decodes API requests
and proxy responses with segmentio, through reflection.

cfjson supports every field type those schemas use, including the ones the
client protocol does not have: `float64`, `[]string`, `map[string]float64`.
Those are covered by the tests in `internal/testtypes`, but not by the
comparison against easyjson, which only has client protocol types: compare
the output for API types when migrating them.

Both schemas have the same recursive type as the client protocol
(`FilterNode`), so they get the nesting limit too.

The same goes for Protobuf: both schemas use vtprotobuf, and
[cfprotobuf](../cfprotobuf) supports every field type they have (`double`
fields and `map<string, double>` included, which the client protocol does not
use). The gRPC side is not affected, the structs stay what `protoc-gen-go`
generates.

### 5. Say which fields are never omitted without rewriting tags

`generate.sh` runs `gomodifytags` eight times to strip `omitempty` from
fields which SDKs expect to be always present (`ClientInfo.user`,
`HistoryResult.publications`, ...). The tags exist only to steer the JSON
generator, so the list could be an input of the generator instead, leaving
`client.pb.go` as `protoc` wrote it.

### 6. Decide the zero-copy rule explicitly

Decided: there is no zero-copy decoding. cfjson always copies, and
`JSONCommandDecoder` copies like the stream decoder does. centrifuge keeps
channel names for the lifetime of a subscription and stores publish data
into history, so a decoded message must never depend on the read buffer.

### 7. HTML escaping

`<`, `>` and `&` in strings are written as `\u003c`, `\u003e` and
`\u0026`.
It is inherited from `encoding/json`, where it protects JSON embedded into
HTML, and does nothing for a WebSocket frame.

Dropping it changes the bytes on the wire. Every JSON parser decodes both
forms to the same string, so it is a compatibility decision rather than a
technical risk, and the gain is small: a few operations less per word of a
string.

### 8. What is left of encoding/json leniency

Decoding is strict about JSON syntax, keys must match field names exactly
and may not repeat. One convention of `encoding/json` is kept: `null` is
accepted for every field and for the message itself, and leaves a scalar
untouched.

Repeated keys are only detected among the fields of a message and the keys
of a map. Inside of a payload (a raw value) and inside of a field the message
does not have, nothing but syntax is checked: the payload belongs to the
application, and finding repeated keys at any depth would take remembering
the keys of every object.

## Elsewhere

### Dropping segmentio from centrifuge

centrifuge imports `github.com/segmentio/encoding/json` directly in five
files (`hub.go`, `client.go`, `client_map.go`, `client_keyed.go`,
`emulation.go`) and three test files, so the dependency stays in the
ecosystem although `protocol` no longer needs it. Everything it is used for
has a replacement here:

| centrifuge does | With cfjson |
| --- | --- |
| `json.Escape(s)`, 14 places: a delta or a payload put into a JSON message as a string | `cfjson.AppendString(make([]byte, 0, len(s)+10), s)` |
| `json.Marshal(cmd)` / `rep` / `push` for trace logging, with a `protojson` fallback | `cmd.AppendJSON(nil)`, which cannot fail |
| `json.Parse(data, &req, json.ZeroCopy)` for `protocol.EmulationRequest` | `cfjson.Unmarshal(data, &req, 0)` |
| `json.Unmarshal(req.Data, &s)` into a string | `cfjson.UnmarshalString(req.Data)` |
| `json.Marshal` and `json.Unmarshal` in tests | `encoding/json` |

Two behaviours differ and need a decision when migrating: `cfjson.Unmarshal`
rejects data after the value where `json.Parse` ignored it, and
`cfjson.UnmarshalString` rejects `null` where `json.Unmarshal` left the
string empty.

`AppendString` produces the same bytes as segmentio's `Escape` for every
input but one: backspace and form feed come out as `\u0008` and `\u000c`
where segmentio writes `\b` and `\f`. Both are the same character to a JSON
parser. This is checked exhaustively by
`TestAppendStringMatchesSegmentioEscape` in `internal/cfjsoncmp`.

Interning decoded strings with the `unique` package was measured and
rejected for the decoder: a string which is already interned costs about as
much as copying it, but one which is not costs about 4 microseconds, and
most strings of a command are chosen by the client. It may still pay off in
centrifuge for channel names, at the point where a validated channel is
stored.

## From a survey of other work

What a look at other parsers, and at what has been written about parsing
untrusted JSON and Protobuf, adds to the list above.

### Repeated keys and letter case (done)

[An Exploration of JSON Interoperability Vulnerabilities](https://bishopfox.com/blog/json-interoperability-vulnerabilities)
(Bishop Fox) catalogues how two parsers which read the same document
differently become a vulnerability: one service validates a message, another
acts on it, and they do not agree on what it says. cfjson was on the safe
side for most of its categories from the start: invalid Unicode (rejected or
replaced, never dropped), numbers which do not fit (an error), NaN and
infinities (never written), comments and trailing data (rejected).

Two were inherited from `encoding/json` and have been changed since: a
repeated key was accepted with the last one winning, and keys were matched
case-insensitively, which made `{"id":1,"ID":2}` a repeated key only this
parser saw as one. Both are errors or non-matches now, as they are by default
in `encoding/json/v2`.

This matters for the Centrifugo HTTP API more than for the client protocol:
its requests are written by hand and by many different clients, and one of
them may well send `{"Channel": ...}`, which segmentio accepts today. For
that the generator has the `-fold-keys` option: a key which is not the name
of a field is matched again without regard to letter case. In
`internal/cfjsoncmp` a decoder generated with it agrees with segmentio's
default matching on random messages with their keys capitalized, and is as
fast as the exact one on input with keys written as fields are named (within
1% on an Apple M4). With every key capitalized it is 23% faster than
segmentio in geomean. A key may still not repeat, in whatever case.

### A conformance suite

[JSONTestSuite](https://github.com/nst/JSONTestSuite) ("Parsing JSON is a
Minefield") is what JSON parsers are measured with: 95 documents which must
be accepted, 188 which must be rejected, 35 which are up to the parser. It is
in `testdata/JSONTestSuite` and `cfjson.Valid` passes all of it, see
`testsuite_test.go` for what it does with the 35.

### Unknown Protobuf fields (done)

A field a message does not know is kept with the message and written back
when it is marshaled: that is what Protobuf implementations do, and what
vtprotobuf did. For a server which decodes commands from clients it means
keeping bytes nobody reads. cfprotobuf has the `-drop-unknown` option for
that, and `protocol` is generated with it.

### Profile-guided optimization

A number of the helpers generated code calls are too large for the compiler
to inline (`cfjson.String`, `cfjson.Skip`, `cfprotobuf.ValidString`). With a
profile from production (`default.pgo` next to `main.go` of Centrifugo) the
compiler inlines the hot ones anyway. This costs nothing in the code here and
is usually worth a few percent overall.

### Ideas looked at and not taken

- **`simd/archsimd`.** Still behind `GOEXPERIMENT=simd` in Go 1.27, the API is
  being revised between releases, and there are open correctness issues
  ([golang/go#81692](https://github.com/golang/go/issues/81692): partial loads
  reading beyond short slices). Worth another look when it is stable, for the
  one loop named in item 2a.
- **A table-driven parser** ([hyperpb](https://mcyoung.xyz/2025/07/16/hyperpb/)).
  Its gains over generated code come from arenas, its own string and map
  representations and a profile of real traffic, none of which fit messages
  which are plain Go structs handed to application code.
- **Interning strings** with the `unique` package: see "Elsewhere" above, where
  it is covered.

### Short strings in the encoder (done)

[goccy/go-json](https://github.com/goccy/go-json/pull/633) writes short
strings without calling `memmove`. `cfjson.AppendString` now does the same
for strings of 4 to 32 bytes when the buffer has room: the words it loads to
check that nothing needs escaping are the words it stores. On an Apple M4 a
string of 4 to 16 bytes takes 2.4 to 2.8 ns where it took 4.7 to 5.4 ns, one
of 17 to 32 bytes 4.5 ns where it took 5.5 to 8.5 ns.

### What other parsers do for speed

A second look, at what fast JSON libraries do which generated pure Go code
could do as well: sonic, goccy/go-json, json-iterator, fastjson, simdjson
and its On-Demand API, yyjson, serde_json, `encoding/json/v2`. Anything
which takes assembly or SIMD, skips a part of validation, changes the bytes
written, or has decoded values share memory with each other or with the
input was out from the start. The numbers are from an Apple M4.

Taken:

- **The key the next field would have** (simdjson On-Demand is fastest when
  fields are asked for in the order they come). Described in the README.
  Decoding got 8.5% faster in geomean for the generated code alone, 10 to 20%
  for messages made of fields rather than of one payload.
- **One hash operation per map entry.** Presence of 100 clients decodes 2 to
  3% faster for it.
- **Long strings checked four words per branch** when encoding. 8 to 12% for
  a string of 40 to 1000 bytes, 3 to 4% for a connect or subscribe command
  with a token in it.
- **An `omitempty` bool is written together with its key**, `,"recover":true`
  in one append: it is only ever written when true. Too small to measure.

Tried and not taken:

- **Looking a key up as a number**: load eight bytes after the quote, find
  the closing quote in the word, compare the masked word with constants.
  Faster than the switch in a benchmark of its own, but on the messages of
  the protocol it gave 3.6% where the simpler comparison above gives 8.5%,
  and it is more code to get right.

Looked at and left, with the reason:

- **Elements of a slice allocated in chunks**, and **all strings of a message
  in one allocation**: fewer allocations (a history of 100 publications
  decodes about 15% faster with the first), but then one publication or one
  string which is kept keeps the memory of all the others alive.
- **A size hint for maps**: needs the entries buffered before the map is
  made. Pays off above eight entries only.
- **A table-driven UTF-8 validator** for text which is not ASCII (twice as
  fast as `utf8.Valid` on arm64 for Cyrillic or emoji): it is a validator of
  our own to keep right, for payloads which are not the common case.
- **Eight digits at a time** when parsing numbers, and **digits written
  straight into the output**: about a nanosecond per number either way.

Already as good as it gets in the standard library, as of Go 1.27: printing
and parsing floats, `strconv.AppendUint`, `utf8.Valid` on ASCII,
`bytes.IndexByte`, copying.

## Moving the other projects over

What was checked against this branch, without changing anything in the other
repositories (a Go workspace pointing at it):

- **centrifuge-go** builds, and its whole test suite passes against a
  Centrifugo built from this branch, which makes it an end-to-end test of
  both sides: the client encodes commands and decodes replies with the new
  code, the server does the opposite.
- **centrifuge** builds. One test fails,
  `TestFrameSizeMetricMeasuresFramesNotCommands`: it writes four JSON commands
  into a frame one after another without the `\n` which separates commands.
  The old decoder took the first and silently ignored the rest, the new one
  rejects data after a command. The test needs the delimiter, no SDK sends
  frames like that.
- **Centrifugo PRO**: both generators accept `apiproto` and `proxyproto`, and
  cfprotobuf accepts `unistream`, `busproto` and the `controlpb` of centrifuge,
  so every field type in use is supported. Only the marshal, unmarshal and
  size features of vtprotobuf are used anywhere, and nothing outside of
  generated files uses easyjson's lexer or writer.

The Protobuf methods of the messages are named after cfprotobuf now:
`MarshalCF`, `SizeCF`, `UnmarshalCF` where they were `MarshalVT`, `SizeVT`,
`UnmarshalVT`. Code which calls them on the types of `protocol` has to be
changed with the move to this version, which is a rename and nothing else:
the methods take and return the same.

What the new packages do not cover:

- **Decoding into `any`.** Centrifugo PRO uses segmentio as a general
  library too. cfjson has no reflection and does not decode into `any`, so
  schemas and dynamic tags, the two places which do that, go to
  `encoding/json/v2`.
- **Structs which are not Protobuf messages** are covered, and without
  changes to the files which declare them, see "Generating without touching
  the files with the types" in the README: the command lives in a file of
  its own and the code goes to another one, so a file shared with the open
  source Centrifugo stays as it is. Tried on the JWT claims of Centrifugo PRO
  (`ConnectTokenClaims` and `SubscribeTokenClaims` of `jwtverify`, with the
  embedded `jwt.RegisteredClaims`, `json.RawMessage` fields and the
  capabilities of another package) with the generated file put in place by a
  build overlay: it compiles, decodes to the same values as segmentio and
  encodes to the same document as `encoding/json`. On an Apple M4, three
  tokens from small to large: decoding takes 23%, 41% and 32% less time,
  encoding 63%, 58% and 70% less. What is left to do there is to call the
  generated code in `token_decoder.go`, or in a file which replaces it in the
  PRO build.
- **Letter case of keys in the HTTP API** is covered by the `-fold-keys`
  option, see "Repeated keys and letter case" above: generate `apiproto` with
  it.
