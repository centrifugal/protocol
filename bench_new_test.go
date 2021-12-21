package protocol

import (
	"io"
	"testing"
)

func newMarshalProtobuf() ([]byte, *Reply, error) {
	r := &Reply{
		Push: &Push{
			Channel: "test",
			Pub: &Publication{
				Data: preparedPayload,
			},
		},
	}
	encoder := NewProtobufReplyEncoder()
	res, err := encoder.Encode(r)
	if err != nil {
		return nil, nil, err
	}
	return res, r, nil
}

func newMarshalJSON() ([]byte, *Reply, error) {
	r := &Reply{
		Push: &Push{
			Channel: "test",
			Pub: &Publication{
				Data: preparedPayload,
			},
		},
	}
	encoder := NewJSONReplyEncoder()
	res, err := encoder.Encode(r)
	if err != nil {
		return nil, nil, err
	}
	return res, r, nil
}

//goland:noinspection GoUnusedGlobalVariable
var benchData []byte

//goland:noinspection GoUnusedGlobalVariable
var benchReply *Reply

//goland:noinspection GoUnusedGlobalVariable
var benchConnectRequest *ConnectRequest

func BenchmarkReplyProtobufMarshalNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, err := newMarshalProtobuf()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufMarshalParallelNew(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, err := newMarshalProtobuf()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
		}
	})
}

func BenchmarkReplyJSONMarshalNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, err := newMarshalJSON()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONMarshalParallelNew(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, err := newMarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
		}
	})
}

func BenchmarkReplyProtobufUnmarshalNew(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := params.MarshalVT()
	cmd := &Command{
		Id:      1,
		Method:  Command_CONNECT,
		Connect: params,
	}
	encoder := NewProtobufCommandEncoder()
	data, _ = encoder.Encode(cmd)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchConnectRequest = unmarshalProtobufNew(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufUnmarshalParallelNew(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := params.MarshalVT()
	cmd := &Command{
		Id:      1,
		Method:  Command_CONNECT,
		Connect: params,
	}
	encoder := NewProtobufCommandEncoder()
	data, _ = encoder.Encode(cmd)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchConnectRequest = unmarshalProtobufNew(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalProtobufNew(b *testing.B, data []byte) *ConnectRequest {
	decoder := GetCommandDecoder(TypeProtobuf, data)
	defer PutCommandDecoder(TypeProtobuf, decoder)
	cmd, err := decoder.Decode()
	if err != nil && err != io.EOF {
		b.Fatal(err)
	}
	if cmd == nil {
		b.Fatal("nil command")
	}
	if cmd.Connect == nil {
		b.Fatal("nil connect")
	}
	if cmd.Connect.Token != "token" {
		b.Fatal()
	}
	return cmd.Connect
}

func BenchmarkReplyJSONUnmarshalNew(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := params.MarshalVT()
	cmd := &Command{
		Id:      1,
		Method:  Command_CONNECT,
		Connect: params,
	}
	encoder := NewJSONCommandEncoder()
	data, _ = encoder.Encode(cmd)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchConnectRequest = unmarshalJSONNew(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONUnmarshalParallelNew(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := params.MarshalVT()
	cmd := &Command{
		Id:      1,
		Method:  Command_CONNECT,
		Connect: params,
	}
	encoder := NewJSONCommandEncoder()
	data, _ = encoder.Encode(cmd)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchConnectRequest = unmarshalJSONNew(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalJSONNew(b *testing.B, data []byte) *ConnectRequest {
	decoder := GetCommandDecoder(TypeJSON, data)
	defer PutCommandDecoder(TypeJSON, decoder)
	cmd, err := decoder.Decode()
	if (err != nil && err != io.EOF) || cmd == nil {
		b.Fatal(err)
	}
	if cmd.Connect == nil {
		b.Fatal("nil connect")
	}
	if cmd.Connect.Token != "token" {
		b.Fatal()
	}
	return cmd.Connect
}
