package protocol

// Tests of what the code generated for the protocol does beyond encoding and
// decoding messages: that it is up to date with its generators, and the rules
// it applies to untrusted input (nesting, trailing data, UTF-8, keys).

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/centrifugal/protocol/cfjson"
	"github.com/centrifugal/protocol/cfjson/gen"
	"github.com/centrifugal/protocol/cfprotobuf"
	pbgen "github.com/centrifugal/protocol/cfprotobuf/gen"
)

// client.pb_cfjson.go must be what the generator produces from client.pb.go
// today: a change of client.proto or of the generator without regenerating
// fails here rather than ships stale JSON code.
func TestGeneratedJSONCodeIsUpToDate(t *testing.T) {
	want, err := gen.Generate(gen.Config{
		Files:          []string{"client.pb.go"},
		RawTypes:       []string{"Raw"},
		ValidRawMethod: "validRaw",
		// As generate.sh runs it, the output file not counted as a file of
		// the package.
		Out: "client.pb_cfjson.go",
	})
	require.NoError(t, err)
	got, err := os.ReadFile("client.pb_cfjson.go")
	require.NoError(t, err)
	require.True(t, bytes.Equal(got, want), "client.pb_cfjson.go is out of date, run make generate")
}

// The same for the Protobuf code.
func TestGeneratedProtobufCodeIsUpToDate(t *testing.T) {
	want, err := pbgen.Generate(pbgen.Config{Files: []string{"client.pb.go", "raw.go"}, DropUnknown: true})
	require.NoError(t, err)
	got, err := os.ReadFile("client.pb_cfprotobuf.go")
	require.NoError(t, err)
	require.True(t, bytes.Equal(got, want), "client.pb_cfprotobuf.go is out of date, run make generate")
}

// Every struct of client.pb.go must be in messageTypes, which the tests of
// every message type go through.
func TestMessageTypesComplete(t *testing.T) {
	src, err := os.ReadFile("client.pb.go")
	require.NoError(t, err)
	count := 0
	for _, line := range strings.Split(string(src), "\n") {
		if name, ok := strings.CutPrefix(line, "type "); ok && strings.HasSuffix(name, " struct {") {
			name = strings.TrimSuffix(name, " struct {")
			require.Contains(t, messageTypes, name)
			count++
		}
	}
	require.Equal(t, len(messageTypes), count)
}

// decodeJSONCommand decodes a frame of one command with both JSON decoders,
// the one of frames and the one of streams, and checks that they agree.
func decodeJSONCommand(t *testing.T, frame string) (*Command, error) {
	t.Helper()
	cmd, err := NewJSONCommandDecoder([]byte(frame)).Decode()
	if errors.Is(err, io.EOF) {
		err = nil
	}
	streamCmd, _, streamErr := NewJSONStreamCommandDecoder(strings.NewReader(frame), 1<<30).Decode()
	if errors.Is(streamErr, io.EOF) {
		streamErr = nil
	}
	require.Equal(t, err == nil, streamErr == nil, "%.100q: the decoders disagree: %v, %v", frame, err, streamErr)
	if err == nil {
		require.Equal(t, cmd, streamCmd)
	} else {
		require.Nil(t, cmd)
		require.Nil(t, streamCmd)
	}
	return cmd, err
}

// FilterNode is the one recursive type of the protocol: a client chooses how
// deep a subscription filter is nested. Decoding must refuse a filter which
// is nested too deep instead of recursing as far as the input goes.
func TestJSONDecode_FilterNodeDepth(t *testing.T) {
	filter := func(depth int) string {
		return `{"id":1,"subscribe":{"channel":"c","tf":` +
			strings.Repeat(`{"op":"and","nodes":[`, depth-1) + `{"op":"eq","key":"k","val":"v"}` + strings.Repeat(`]}`, depth-1) + `}}`
	}
	depthOf := func(cmd *Command) int {
		depth := 1
		for node := cmd.Subscribe.Tf; len(node.Nodes) > 0; node = node.Nodes[0] {
			depth++
		}
		return depth
	}
	for _, depth := range []int{1, 3, cfjson.MaxDepth} {
		cmd, err := decodeJSONCommand(t, filter(depth))
		require.NoError(t, err)
		require.Equal(t, depth, depthOf(cmd))
	}
	for _, depth := range []int{cfjson.MaxDepth + 1, 1000000} {
		_, err := decodeJSONCommand(t, filter(depth))
		require.Error(t, err)
	}
}

// A command is one JSON value: bytes after it are not ignored. Whitespace is
// not data.
func TestJSONDecode_TrailingData(t *testing.T) {
	for _, frame := range []string{`{"id":1}x`, `{"id":1}{"id":2}`, `{"id":1},`, `{"id":1} null`} {
		_, err := decodeJSONCommand(t, frame)
		require.ErrorIs(t, err, cfjson.ErrTrailingData, frame)
	}
	for _, frame := range []string{"{\"id\":1} ", " \t{\"id\":1}\r", "{\"id\":1}\r\n"} {
		cmd, err := decodeJSONCommand(t, frame)
		require.NoError(t, err, frame)
		require.Equal(t, uint32(1), cmd.Id)
	}
}

// JSON is UTF-8 (RFC 8259), and a WebSocket text frame which is not makes a
// browser drop the connection. So invalid UTF-8 is neither accepted from a
// client, where it would end up in frames sent to subscribers, nor let out by
// the encoders when it comes from the application.
func TestJSONDecode_InvalidUTF8(t *testing.T) {
	for _, frame := range []string{
		"{\"subscribe\":{\"channel\":\"bad\xff\"}}",
		"{\"publish\":{\"channel\":\"c\",\"data\":{\"text\":\"bad\xff\"}}}",
		"{\"publish\":{\"channel\":\"c\",\"data\":\"trunc\xc3\"}}",
		"{\"unknown\":\"\xed\xa0\x80\"}",
		"{\"bad\xffkey\":1}",
	} {
		_, err := decodeJSONCommand(t, frame)
		require.Error(t, err, frame)
	}
	cmd, err := decodeJSONCommand(t, "{\"publish\":{\"channel\":\"caf\xc3\xa9\",\"data\":{\"text\":\"\xe2\x9c\x93 \xf0\x9f\x98\x80\"}}}")
	require.NoError(t, err)
	require.Equal(t, "caf\xc3\xa9", cmd.Publish.Channel)
	require.Equal(t, "{\"text\":\"\xe2\x9c\x93 \xf0\x9f\x98\x80\"}", string(cmd.Publish.Data))

	// A payload of the application is refused.
	bad := Raw("{\"text\":\"bad\xff\"}")
	_, err = NewJSONReplyEncoder().Encode(&Reply{Push: &Push{Channel: "c", Pub: &Publication{Data: bad}}})
	require.ErrorIs(t, err, errInvalidJSON)
	// A string field is made valid, whatever it holds.
	data, err := NewJSONReplyEncoder().Encode(&Reply{Error: &Error{Code: 1, Message: "bad\xff"}})
	require.NoError(t, err)
	require.Equal(t, `{"error":{"code":1,"message":"bad`+"\\"+`ufffd"}}`, string(data))
}

// A key is matched to a field exactly, and may come once. Both rules are
// there so that this parser cannot read a message differently from another
// one which has looked at the same bytes: a proxy, a firewall, a client.
func TestJSONDecode_Keys(t *testing.T) {
	for _, frame := range []string{
		`{"id":1,"id":2}`,
		`{"id":1,"connect":{},"id":1}`,
		`{"publish":{"channel":"a","channel":"b"}}`,
		`{"publish":{"data":{},"data":{}}}`,
		`{"connect":{"subs":{"c":{},"c":{}}}}`,
		`{"connect":{"headers":{"k":"a","k":"b"}}}`,
		`{"subscribe":{"channel":"c"},"subscribe":{"channel":"d"}}`,
	} {
		_, err := decodeJSONCommand(t, frame)
		require.Error(t, err, frame)
	}
	// A key in another case is a field the message does not have.
	cmd, err := decodeJSONCommand(t, `{"ID":5,"Publish":{"channel":"x"},"id":1,"publish":{"Channel":"y","channel":"c"}}`)
	require.NoError(t, err)
	require.Equal(t, uint32(1), cmd.Id)
	require.Equal(t, "c", cmd.Publish.Channel)

	// Inside of a payload nothing is looked at beyond its syntax: its keys
	// are the business of whoever reads it.
	cmd, err = decodeJSONCommand(t, `{"publish":{"channel":"c","data":{"a":1,"a":2,"A":3}}}`)
	require.NoError(t, err)
	require.Equal(t, `{"a":1,"a":2,"A":3}`, string(cmd.Publish.Data))
}

// null is a value a pointer may have, and it stays nil when it is an element
// of a slice or of a map, as encoding/json has it: whoever walks over a
// decoded message has to be ready for it.
func TestJSONDecode_NullElements(t *testing.T) {
	cmd, err := decodeJSONCommand(t, `{"id":1,"connect":{"subs":{"a":null,"b":{"recover":true}}}}`)
	require.NoError(t, err)
	require.Len(t, cmd.Connect.Subs, 2)
	require.Nil(t, cmd.Connect.Subs["a"])
	require.True(t, cmd.Connect.Subs["b"].Recover)
}

// The same limit applies to the Protobuf encoding of a filter.
func TestProtobufDecode_FilterNodeDepth(t *testing.T) {
	filter := func(depth int) []byte {
		// Built from the inside out: every level is a node holding the
		// previous one in its nodes field.
		node := &FilterNode{Op: "eq", Key: "k", Val: "v"}
		data, err := node.MarshalCF()
		require.NoError(t, err)
		for ; depth > 1; depth-- {
			// Field 6 (nodes), length-delimited.
			wrapped := append([]byte{0x32}, protoLength(len(data))...)
			data = append(wrapped, data...)
		}
		return data
	}
	var node FilterNode
	require.NoError(t, node.UnmarshalCF(filter(cfprotobuf.MaxDepth)))
	depth := 1
	for n := &node; len(n.Nodes) > 0; n = n.Nodes[0] {
		depth++
	}
	require.Equal(t, cfprotobuf.MaxDepth, depth)

	require.ErrorIs(t, new(FilterNode).UnmarshalCF(filter(cfprotobuf.MaxDepth+1)), cfprotobuf.ErrTooDeep)

	// Inside of a command, through the decoder.
	cmd := append([]byte{0x6a}, protoLength(len(filter(cfprotobuf.MaxDepth+1)))...) // SubscribeRequest.tf
	cmd = append(cmd, filter(cfprotobuf.MaxDepth+1)...)
	cmd = append(append([]byte{0x2a}, protoLength(len(cmd))...), cmd...) // Command.subscribe
	frame := append(protoLength(len(cmd)), cmd...)
	decoded, err := NewProtobufCommandDecoder(frame).Decode()
	require.ErrorIs(t, err, cfprotobuf.ErrTooDeep)
	require.Nil(t, decoded)
}

// protoLength returns n encoded as a varint.
func protoLength(n int) []byte {
	var out []byte
	for n >= 0x80 {
		out = append(out, byte(n)|0x80)
		n >>= 7
	}
	return append(out, byte(n))
}

// jsonPayloads are values random payloads are given for the JSON encoding,
// which writes a payload as it is and so needs it to be JSON.
var jsonPayloads = []string{
	`null`, `0`, `-1.5e10`, `""`, `"h\u00e9llo \ud83d\ude00"`, `[]`, `[1,"a",null,true]`,
	`{}`, `{"a":{"b":[{"c":"<&>"}]}}`, `{ "spaced" : [ 1 ] }`,
}

// setJSONPayloads replaces every payload in v which is not empty with a JSON
// value.
func setJSONPayloads(r *rand.Rand, v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			setJSONPayloads(r, v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				setJSONPayloads(r, v.Field(i))
			}
		}
	case reflect.Slice:
		if v.Type() == reflect.TypeFor[Raw]() {
			if v.Len() > 0 {
				v.SetBytes([]byte(jsonPayloads[r.Intn(len(jsonPayloads))]))
			}
			return
		}
		for i := 0; i < v.Len(); i++ {
			setJSONPayloads(r, v.Index(i))
		}
	case reflect.Map:
		for iter := v.MapRange(); iter.Next(); {
			elem := reflect.New(v.Type().Elem()).Elem()
			elem.Set(iter.Value())
			setJSONPayloads(r, elem)
			v.SetMapIndex(iter.Key(), elem)
		}
	}
}

// What the encoders of the package write, its decoders read back as the same
// message: for every message type, with random values, in both encodings.
// proto.Equal does not tell an empty slice, map or payload from a missing
// one, which encoders may drop.
func TestRoundTrip(t *testing.T) {
	forEachType(t, func(t *testing.T, protoType Type) {
		r := rand.New(rand.NewSource(7))
		for n := 0; n < 20000; n++ {
			name, m := randomPBMessage(r, 3)
			var got message
			if protoType == TypeJSON {
				setJSONPayloads(r, reflect.ValueOf(m))
				data := m.AppendJSON(nil)
				require.True(t, cfjson.Valid(data), "%s: %s", name, data)
				got = newMessage(name)
				// Payloads are written as they are and come back as they
				// were written, spaces inside of them too (spaces around a
				// value are not a part of it, and do not come back).
				require.NoError(t, cfjson.Unmarshal(data, got, 0), "%s: %s", name, data)
			} else {
				data, err := m.MarshalCF()
				require.NoError(t, err, name)
				got = newMessage(name)
				require.NoError(t, got.UnmarshalCF(data), "%s: %x", name, data)
			}
			require.True(t, proto.Equal(m, got), "%s:\nencoded %v\ndecoded %v", name, m, got)
		}
	})
}

// The two encodings of a message mean the same: a message which goes through
// JSON and one which goes through Protobuf come out equal. A field the
// generator lost or got wrong in one of the codecs shows up here.
func TestCrossFormat(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for n := 0; n < 20000; n++ {
		name, m := randomPBMessage(r, 3)
		setJSONPayloads(r, reflect.ValueOf(m))

		viaJSON := newMessage(name)
		require.NoError(t, cfjson.Unmarshal(m.AppendJSON(nil), viaJSON, 0), name)

		data, err := m.MarshalCF()
		require.NoError(t, err, name)
		viaProtobuf := newMessage(name)
		require.NoError(t, viaProtobuf.UnmarshalCF(data), name)

		require.True(t, proto.Equal(viaJSON, viaProtobuf), "%s:\nvia JSON     %v\nvia Protobuf %v", name, viaJSON, viaProtobuf)
	}
}

// generate.sh is what generates the code: the freshness tests above call the
// generators as libraries, this runs their commands with the very flags of
// generate.sh and compares what they write with the files in the repository.
func TestGenerateScript(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the generators")
	}
	script, err := os.ReadFile("generate.sh")
	require.NoError(t, err)
	commands := 0
	for _, line := range strings.Split(string(script), "\n") {
		if !strings.HasPrefix(line, "go run ./cfjson/cmd/cfjson ") && !strings.HasPrefix(line, "go run ./cfprotobuf/cmd/cfprotobuf ") {
			continue
		}
		commands++
		args := strings.Fields(line)[2:]
		var committed string
		for i := range args {
			if args[i] == "-out" {
				committed = args[i+1]
				args[i+1] = filepath.Join(t.TempDir(), committed)
			}
		}
		require.NotEmpty(t, committed, line)
		out, err := exec.Command("go", append([]string{"run"}, args...)...).CombinedOutput() //nolint:gosec // G204: the commands of generate.sh.
		require.NoError(t, err, "%s: %s", line, out)

		var written string
		for i := range args {
			if args[i] == "-out" {
				written = args[i+1]
			}
		}
		want, err := os.ReadFile(committed) //nolint:gosec // G304: a file generate.sh names.
		require.NoError(t, err)
		got, err := os.ReadFile(written) //nolint:gosec // G304: a file of t.TempDir.
		require.NoError(t, err)
		require.True(t, bytes.Equal(want, got), "%s differs from what generate.sh writes, run make generate", committed)
	}
	require.Equal(t, 2, commands, "generate.sh has no longer the commands this test runs")
}
