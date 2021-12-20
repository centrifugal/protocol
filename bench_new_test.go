package protocol

import (
	"io"
	"testing"
)

func newMarshalProtobuf() ([]byte, error) {
	r := &Reply{
		Push: &Push{
			Channel: "test",
			Pub: &Publication{
				Data: preparedPayload,
			},
		},
	}
	encoder := NewProtobufReplyEncoder()
	return encoder.Encode(r)
}

func newMarshalJSON() ([]byte, error) {
	r := &Reply{
		Push: &Push{
			Channel: "test",
			Pub: &Publication{
				Data: preparedPayload,
			},
		},
	}
	encoder := NewJSONReplyEncoder()
	return encoder.Encode(r)
}

func BenchmarkReplyProtobufMarshalNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := newMarshalProtobuf()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufMarshalParallelNew(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := newMarshalProtobuf()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkReplyJSONMarshalNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := newMarshalJSON()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONMarshalParallelNew(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := newMarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
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
		unmarshalProtobufNew(b, data)
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
			unmarshalProtobufNew(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalProtobufNew(b *testing.B, data []byte) {
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
		unmarshalJSONNew(b, data)
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
			unmarshalJSONNew(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalJSONNew(b *testing.B, data []byte) {
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
}
