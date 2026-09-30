package protocol

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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
		Files:    []string{"client.pb.go"},
		RawTypes: []string{"Raw"},
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

// FilterNode is the one recursive type of the protocol: a client chooses how
// deep a subscription filter is nested. Decoding must refuse a filter which
// is nested too deep instead of recursing as far as the input goes.
func TestJSONDecode_FilterNodeDepth(t *testing.T) {
	filter := func(depth int) []byte {
		return []byte(`{"id":1,"subscribe":{"channel":"c","tf":` +
			strings.Repeat(`{"op":"and","nodes":[`, depth-1) + `{"op":"eq","key":"k","val":"v"}` + strings.Repeat(`]}`, depth-1) + `}}`)
	}
	depthOf := func(cmd *Command) int {
		depth := 0
		for node := cmd.Subscribe.Tf; node != nil; depth++ {
			if len(node.Nodes) == 0 {
				return depth + 1
			}
			node = node.Nodes[0]
		}
		return depth
	}

	cmd, err := NewJSONCommandDecoder(filter(3)).Decode()
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, 3, depthOf(cmd))

	cmd, err = NewJSONCommandDecoder(filter(cfjson.MaxDepth)).Decode()
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, cfjson.MaxDepth, depthOf(cmd))

	for _, depth := range []int{cfjson.MaxDepth + 1, 1000000} {
		cmd, err = NewJSONCommandDecoder(filter(depth)).Decode()
		require.Error(t, err)
		require.Nil(t, cmd)

		decoder := NewJSONStreamCommandDecoder(bytes.NewReader(filter(depth)), 1<<30)
		cmd, _, err = decoder.Decode()
		require.Error(t, err)
		require.Nil(t, cmd)
	}
}

// A command must be one JSON value: bytes after it are not ignored.
func TestJSONDecode_TrailingData(t *testing.T) {
	for _, frame := range []string{`{"id":1}x`, `{"id":1}{"id":2}`, `{"id":1},`, `{"id":1} null`} {
		cmd, err := NewJSONCommandDecoder([]byte(frame)).Decode()
		require.ErrorIs(t, err, cfjson.ErrTrailingData, frame)
		require.Nil(t, cmd)

		cmd, _, err = NewJSONStreamCommandDecoder(strings.NewReader(frame), 1024).Decode()
		require.ErrorIs(t, err, cfjson.ErrTrailingData, frame)
		require.Nil(t, cmd)
	}
	// Whitespace is not data.
	for _, frame := range []string{"{\"id\":1} ", " \t{\"id\":1}\r", "{\"id\":1}\r\n"} {
		cmd, err := NewJSONCommandDecoder([]byte(frame)).Decode()
		require.ErrorIs(t, err, io.EOF, frame)
		require.Equal(t, uint32(1), cmd.Id)

		cmd, _, err = NewJSONStreamCommandDecoder(strings.NewReader(frame), 1024).Decode()
		if err != nil {
			require.ErrorIs(t, err, io.EOF, frame)
		}
		require.Equal(t, uint32(1), cmd.Id)
	}
}

// JSON is UTF-8 (RFC 8259), and a WebSocket text frame which is not makes a
// browser drop the connection. So invalid UTF-8 must neither be accepted from
// a client, where it would end up in frames sent to subscribers, nor leave
// the encoders when it comes from the application.
func TestJSON_InvalidUTF8(t *testing.T) {
	for _, frame := range []string{
		"{\"subscribe\":{\"channel\":\"bad\xff\"}}",
		"{\"publish\":{\"channel\":\"c\",\"data\":{\"text\":\"bad\xff\"}}}",
		"{\"publish\":{\"channel\":\"c\",\"data\":\"trunc\xc3\"}}",
		"{\"unknown\":\"\xed\xa0\x80\"}",
		"{\"bad\xffkey\":1}",
	} {
		cmd, err := NewJSONCommandDecoder([]byte(frame)).Decode()
		require.Error(t, err, frame)
		require.Nil(t, cmd)
	}
	// Valid UTF-8 is of course fine, in fields and in payloads.
	cmd, err := NewJSONCommandDecoder([]byte("{\"publish\":{\"channel\":\"caf\xc3\xa9\",\"data\":{\"text\":\"\xe2\x9c\x93 \xf0\x9f\x98\x80\"}}}")).Decode()
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, "caf\xc3\xa9", cmd.Publish.Channel)
	require.Equal(t, "{\"text\":\"\xe2\x9c\x93 \xf0\x9f\x98\x80\"}", string(cmd.Publish.Data))

	// A payload from the application.
	bad := Raw("{\"text\":\"bad\xff\"}")
	_, err = NewJSONReplyEncoder().Encode(&Reply{Push: &Push{Channel: "c", Pub: &Publication{Data: bad}}})
	require.ErrorIs(t, err, errInvalidJSON)
	_, err = NewJSONPushEncoder().Encode(&Push{Channel: "c", Pub: &Publication{Data: bad}})
	require.ErrorIs(t, err, errInvalidJSON)

	// Invalid UTF-8 in a string field of a message is replaced when encoding,
	// so the output is valid whatever the field holds.
	res, err := NewJSONReplyEncoder().Encode(&Reply{Error: &Error{Code: 1, Message: "bad\xff"}})
	require.NoError(t, err)
	require.Equal(t, `{"error":{"code":1,"message":"bad`+"\\"+`ufffd"}}`, string(res))
}

// A Raw which has nothing but newlines in it must not leave a hole in the
// message.
func TestJSONEncode_RawOfNewlines(t *testing.T) {
	res, err := NewJSONReplyEncoder().Encode(&Reply{Push: &Push{Channel: "c", Pub: &Publication{Data: Raw("\n\n")}}})
	require.NoError(t, err)
	require.Equal(t, `{"push":{"channel":"c","pub":{"data":null}}}`, string(res))
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

// A key is matched to a field exactly, and may come once. Both rules are
// there so that this parser cannot read a message differently from another
// one which has looked at the same bytes: a proxy, a firewall, a client.
func TestJSONDecode_KeysAreExactAndUnique(t *testing.T) {
	for _, frame := range []string{
		`{"id":1,"id":2}`,
		`{"id":1,"connect":{},"id":1}`,
		`{"publish":{"channel":"a","channel":"b"}}`,
		`{"publish":{"data":{},"data":{}}}`,
		`{"connect":{"subs":{"c":{},"c":{}}}}`,
		`{"connect":{"headers":{"k":"a","k":"b"}}}`,
		`{"subscribe":{"channel":"c"},"subscribe":{"channel":"d"}}`,
	} {
		cmd, err := NewJSONCommandDecoder([]byte(frame)).Decode()
		require.Error(t, err, frame)
		require.Nil(t, cmd)

		cmd, _, err = NewJSONStreamCommandDecoder(strings.NewReader(frame), 1024).Decode()
		require.Error(t, err, frame)
		require.Nil(t, cmd)
	}
	// A key in another case is a field the message does not have.
	cmd, err := NewJSONCommandDecoder([]byte(`{"ID":5,"Publish":{"channel":"x"},"id":1,"publish":{"Channel":"y","channel":"c"}}`)).Decode()
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, uint32(1), cmd.Id)
	require.Equal(t, "c", cmd.Publish.Channel)

	// Inside of a payload nothing is looked at beyond its syntax: what keys
	// it has is the business of whoever reads it.
	cmd, err = NewJSONCommandDecoder([]byte(`{"publish":{"channel":"c","data":{"a":1,"a":2,"A":3}}}`)).Decode()
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, `{"a":1,"a":2,"A":3}`, string(cmd.Publish.Data))
}

// null is a value a pointer may have in JSON, and it stays nil when it is an
// element of a slice or of a map too, as encoding/json has it: whoever walks
// over a decoded message has to be ready for it.
func TestJSONNullElementsAreNil(t *testing.T) {
	var cmd Command
	require.NoError(t, cfjson.Unmarshal([]byte(`{"id":1,"connect":{"subs":{"a":null,"b":{"recover":true}}}}`), &cmd, 0))
	require.Len(t, cmd.Connect.Subs, 2)
	require.Nil(t, cmd.Connect.Subs["a"])
	require.True(t, cmd.Connect.Subs["b"].Recover)
}

// nilElement looks for a nil message in the slices and maps of a message and
// returns the name of the field which has it.
func nilElement(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			return nilElement(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			if where := nilElement(v.Field(i)); where != "" {
				return f.Name + "." + where
			}
		}
	case reflect.Slice, reflect.Map:
		if v.Type().Elem().Kind() != reflect.Pointer {
			return ""
		}
		if v.Kind() == reflect.Slice {
			for i := 0; i < v.Len(); i++ {
				if v.Index(i).IsNil() {
					return "nil"
				}
				if where := nilElement(v.Index(i)); where != "" {
					return where
				}
			}
			return ""
		}
		for iter := v.MapRange(); iter.Next(); {
			if iter.Value().IsNil() {
				return "nil"
			}
			if where := nilElement(iter.Value()); where != "" {
				return where
			}
		}
	}
	return ""
}
