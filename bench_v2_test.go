package protocol

import (
	"io"
	"testing"
)

func marshalProtobufV2() ([]byte, *Reply, error) {
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

func marshalJSONV2() ([]byte, *Reply, error) {
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

func BenchmarkReplyProtobufMarshalV2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, err := marshalProtobufV2()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufMarshalParallelV2(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, err := marshalProtobufV2()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
		}
	})
}

func BenchmarkReplyJSONMarshalV2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, err := marshalJSONV2()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONMarshalParallelV2(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, err := marshalJSONV2()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
		}
	})
}

func BenchmarkReplyProtobufUnmarshalV2(b *testing.B) {
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
		benchConnectRequest = unmarshalProtobufV2(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufUnmarshalParallelV2(b *testing.B) {
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
			benchConnectRequest = unmarshalProtobufV2(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalProtobufV2(b *testing.B, data []byte) *ConnectRequest {
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

func BenchmarkReplyJSONUnmarshalV2(b *testing.B) {
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
		benchConnectRequest = unmarshalJSONV2(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONUnmarshalParallelV2(b *testing.B) {
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
			benchConnectRequest = unmarshalJSONV2(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalJSONV2(b *testing.B, data []byte) *ConnectRequest {
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
