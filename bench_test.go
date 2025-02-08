package protocol

import (
	"io"
	"testing"
)

func benchPayload() []byte {
	size := 256
	var p []byte
	for i := 0; i < size; i++ {
		p = append(p, 'i')
	}
	return []byte(`{"input":"` + string(p) + `"}`)
}

var preparedPayload = benchPayload()

func marshalProtobufConnect(reply *Reply) ([]byte, error) {
	encoder := DefaultProtobufReplyEncoder
	res, err := encoder.Encode(reply)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func marshalProtobufConnectNoCopy(reply *Reply) ([]byte, func(), error) {
	encoder := DefaultProtobufReplyEncoder
	res, releaseFn, err := encoder.EncodeNoCopy(reply)
	if err != nil {
		return nil, nil, err
	}
	return res, releaseFn, nil
}

func marshalProtobuf() ([]byte, *Reply, error) {
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

func marshalProtobufNoCopy() ([]byte, *Reply, func(), error) {
	r := &Reply{
		Push: &Push{
			Channel: "test",
			Pub: &Publication{
				Data: preparedPayload,
			},
		},
	}
	encoder := NewProtobufReplyEncoder()
	res, doneFn, err := encoder.EncodeNoCopy(r)
	if err != nil {
		return nil, nil, nil, err
	}
	return res, r, doneFn, nil
}

func marshalJSON() ([]byte, *Reply, error) {
	r := &Reply{
		Push: &Push{
			Channel: "test",
			Pub: &Publication{
				Data: preparedPayload,
			},
		},
	}
	res, err := DefaultJsonReplyEncoder.Encode(r)
	if err != nil {
		return nil, nil, err
	}
	return res, r, nil
}

func marshalJSONNoCopy() ([]byte, *Reply, func(), error) {
	r := &Reply{
		Push: &Push{
			Channel: "test",
			Pub: &Publication{
				Data: preparedPayload,
			},
		},
	}
	res, doneFn, err := DefaultJsonReplyEncoder.EncodeNoCopy(r)
	if err != nil {
		return nil, nil, nil, err
	}
	return res, r, doneFn, nil
}

func marshalJSONConnect(reply *Reply) ([]byte, error) {
	encoder := DefaultJsonReplyEncoder
	res, err := encoder.Encode(reply)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func marshalJSONConnectNoCopy(reply *Reply) ([]byte, func(), error) {
	encoder := DefaultJsonReplyEncoder
	res, releaseFn, err := encoder.EncodeNoCopy(reply)
	if err != nil {
		return nil, nil, err
	}
	return res, releaseFn, nil
}

//goland:noinspection GoUnusedGlobalVariable
var benchData []byte

//goland:noinspection GoUnusedGlobalVariable
var benchReply *Reply

//goland:noinspection GoUnusedGlobalVariable
var benchConnectRequest *ConnectRequest

func BenchmarkReplyMarshalProtobuf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, err := marshalProtobuf()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
	}
	b.ReportAllocs()
}

func BenchmarkReplyMarshalProtobufNoCopy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, doneFn, err := marshalProtobufNoCopy()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
		doneFn()
	}
	b.ReportAllocs()
}

// This is how we write command replies in Centrifuge.
func BenchmarkReplyMarshalProtobufConnect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		res := ConnectResultFromVTPool()
		res.Client = "clientclientclientclientclientclientclientclientclientclient"
		res.Version = "0.0.0"
		res.Ping = 25
		res.Pong = true
		r := ReplyPool.AcquireConnectReply(res)
		d, err := marshalProtobufConnect(r)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		ReplyPool.ReleaseConnectReply(r)
		res.ReturnToVTPool()
	}
	b.ReportAllocs()
}

// This is how we write command replies in Centrifuge.
func BenchmarkReplyMarshalProtobufConnectNoCopy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		res := ConnectResultFromVTPool()
		res.Client = "clientclientclientclientclientclientclientclientclientclient"
		res.Version = "0.0.0"
		res.Ping = 25
		res.Pong = true
		r := ReplyPool.AcquireConnectReply(res)
		d, releaseFn, err := marshalProtobufConnectNoCopy(r)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		releaseFn()
		ReplyPool.ReleaseConnectReply(r)
		res.ReturnToVTPool()
	}
	b.ReportAllocs()
}

func BenchmarkReplyMarshalProtobufParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, err := marshalProtobuf()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
		}
	})
}

func BenchmarkReplyMarshalProtobufNoCopyParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, doneFn, err := marshalProtobufNoCopy()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
			doneFn()
		}
	})
}

func BenchmarkReplyMarshalProtobufConnectParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := ConnectResultFromVTPool()
			res.Client = "clientclientclientclientclientclientclientclientclientclient"
			res.Version = "0.0.0"
			res.Ping = 25
			res.Pong = true
			r := ReplyPool.AcquireConnectReply(res)
			d, err := marshalProtobufConnect(r)
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			ReplyPool.ReleaseConnectReply(r)
			res.ReturnToVTPool()
		}
	})
}

func BenchmarkReplyMarshalProtobufConnectNoCopyParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := ConnectResultFromVTPool()
			res.Client = "clientclientclientclientclientclientclientclientclientclient"
			res.Version = "0.0.0"
			res.Ping = 25
			res.Pong = true
			r := ReplyPool.AcquireConnectReply(res)
			d, releaseFn, err := marshalProtobufConnectNoCopy(r)
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			releaseFn()
			ReplyPool.ReleaseConnectReply(r)
			res.ReturnToVTPool()
		}
	})
}

func BenchmarkReplyMarshalJSON(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, err := marshalJSON()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
	}
	b.ReportAllocs()
}

func BenchmarkReplyMarshalJSONNoCopy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d, r, doneFn, err := marshalJSONNoCopy()
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		benchReply = r
		doneFn()
	}
	b.ReportAllocs()
}

func BenchmarkReplyMarshalJSONConnect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		res := ConnectResultFromVTPool()
		res.Client = "clientclientclientclientclientclientclientclientclientclient"
		res.Version = "0.0.0"
		res.Ping = 25
		res.Pong = true
		r := ReplyPool.AcquireConnectReply(res)
		d, err := marshalJSONConnect(r)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		ReplyPool.ReleaseConnectReply(r)
		res.ReturnToVTPool()
	}
	b.ReportAllocs()
}

func BenchmarkReplyMarshalJSONConnectNoCopy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		res := ConnectResultFromVTPool()
		res.Client = "clientclientclientclientclientclientclientclientclientclient"
		res.Version = "0.0.0"
		res.Ping = 25
		res.Pong = true
		r := ReplyPool.AcquireConnectReply(res)
		d, releaseFn, err := marshalJSONConnectNoCopy(r)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
		releaseFn()
		ReplyPool.ReleaseConnectReply(r)
		res.ReturnToVTPool()
	}
	b.ReportAllocs()
}

func BenchmarkReplyMarshalJSONParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, err := marshalJSON()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
		}
	})
}

func BenchmarkReplyMarshalJSONNoCopyParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d, r, doneFn, err := marshalJSONNoCopy()
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			benchReply = r
			doneFn()
		}
	})
}

func BenchmarkReplyMarshalJSONConnectParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := ConnectResultFromVTPool()
			res.Client = "clientclientclientclientclientclientclientclientclientclient"
			res.Version = "0.0.0"
			res.Ping = 25
			res.Pong = true
			r := ReplyPool.AcquireConnectReply(res)
			d, err := marshalJSONConnect(r)
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			ReplyPool.ReleaseConnectReply(r)
			res.ReturnToVTPool()
		}
	})
}

func BenchmarkReplyMarshalJSONConnectNoCopyParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := ConnectResultFromVTPool()
			res.Client = "clientclientclientclientclientclientclientclientclientclient"
			res.Version = "0.0.0"
			res.Ping = 25
			res.Pong = true
			r := ReplyPool.AcquireConnectReply(res)
			d, releaseFn, err := marshalJSONConnectNoCopy(r)
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
			releaseFn()
			ReplyPool.ReleaseConnectReply(r)
			res.ReturnToVTPool()
		}
	})
}

func BenchmarkReplyProtobufUnmarshal(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	cmd := &Command{
		Id:      1,
		Connect: params,
	}
	encoder := NewProtobufCommandEncoder()
	data, _ := encoder.Encode(cmd)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchConnectRequest = unmarshalProtobuf(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufUnmarshalParallel(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	cmd := &Command{
		Id:      1,
		Connect: params,
	}
	encoder := NewProtobufCommandEncoder()
	data, _ := encoder.Encode(cmd)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchConnectRequest = unmarshalProtobuf(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalProtobuf(b *testing.B, data []byte) *ConnectRequest {
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

func BenchmarkReplyJSONUnmarshal(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	cmd := &Command{
		Id:      1,
		Connect: params,
	}
	encoder := NewJSONCommandEncoder()
	data, _ := encoder.Encode(cmd)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchConnectRequest = unmarshalJSON(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONUnmarshalParallel(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	cmd := &Command{
		Id:      1,
		Connect: params,
	}
	encoder := NewJSONCommandEncoder()
	data, _ := encoder.Encode(cmd)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchConnectRequest = unmarshalJSON(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalJSON(b *testing.B, data []byte) *ConnectRequest {
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
