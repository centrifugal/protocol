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

//func marshalProtobufConnect(reply *Reply) ([]byte, error) {
//	encoder := DefaultProtobufReplyEncoder
//	res, err := encoder.Encode(reply)
//	if err != nil {
//		return nil, err
//	}
//	return res, nil
//}

//func marshalProtobufConnectNoCopy(reply *Reply, buf []byte) ([]byte, error) {
//	encoder := DefaultProtobufReplyEncoder
//	res, err := encoder.EncodeNoCopy(reply, buf)
//	if err != nil {
//		return nil, err
//	}
//	return res, nil
//}

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

//func marshalJSONConnect(reply *Reply) ([]byte, error) {
//	encoder := DefaultJsonReplyEncoder
//	res, err := encoder.Encode(reply)
//	if err != nil {
//		return nil, err
//	}
//	return res, nil
//}

//func marshalJSONConnectNoCopy(reply *Reply, buf []byte) ([]byte, error) {
//	encoder := DefaultJsonReplyEncoder
//	res, err := encoder.EncodeNoCopy(reply, buf)
//	if err != nil {
//		return nil, err
//	}
//	return res, nil
//}

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

//// This is how we write command replies in Centrifuge.
//func BenchmarkReplyMarshalProtobufConnect(b *testing.B) {
//	for i := 0; i < b.N; i++ {
//		res := ConnectResultFromVTPool()
//		res.Client = "clientclientclientclientclientclientclientclientclientclient"
//		res.Version = "0.0.0"
//		res.Ping = 25
//		res.Pong = true
//		r := ReplyPool.AcquireConnectReply(res)
//		d, err := marshalProtobufConnect(r)
//		if err != nil {
//			b.Fatal(err)
//		}
//		benchData = d
//		ReplyPool.ReleaseConnectReply(r)
//		res.ReturnToVTPool()
//	}
//	b.ReportAllocs()
//}

//// This is how we write command replies in Centrifuge.
//func BenchmarkReplyMarshalProtobufConnectNoCopy(b *testing.B) {
//	for i := 0; i < b.N; i++ {
//		res := ConnectResultFromVTPool()
//		res.Client = "clientclientclientclientclientclientclientclientclientclient"
//		res.Version = "0.0.0"
//		res.Ping = 25
//		res.Pong = true
//		r := ReplyPool.AcquireConnectReply(res)
//		buf := getByteBuffer(r.SizeVT())
//		d, err := marshalProtobufConnectNoCopy(r, buf.B)
//		if err != nil {
//			b.Fatal(err)
//		}
//		benchData = d
//		putByteBuffer(buf)
//		ReplyPool.ReleaseConnectReply(r)
//		res.ReturnToVTPool()
//	}
//	b.ReportAllocs()
//}

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

//func BenchmarkReplyMarshalProtobufConnectParallel(b *testing.B) {
//	b.ReportAllocs()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			res := ConnectResultFromVTPool()
//			res.Client = "clientclientclientclientclientclientclientclientclientclient"
//			res.Version = "0.0.0"
//			res.Ping = 25
//			res.Pong = true
//			r := ReplyPool.AcquireConnectReply(res)
//			d, err := marshalProtobufConnect(r)
//			if err != nil {
//				b.Fatal(err)
//			}
//			benchData = d
//			ReplyPool.ReleaseConnectReply(r)
//			res.ReturnToVTPool()
//		}
//	})
//}

//func BenchmarkReplyMarshalProtobufConnectNoCopyParallel(b *testing.B) {
//	b.ReportAllocs()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			res := ConnectResultFromVTPool()
//			res.Client = "clientclientclientclientclientclientclientclientclientclient"
//			res.Version = "0.0.0"
//			res.Ping = 25
//			res.Pong = true
//			r := ReplyPool.AcquireConnectReply(res)
//			buf := getByteBuffer(r.SizeVT())
//			d, err := marshalProtobufConnectNoCopy(r, buf.B)
//			if err != nil {
//				b.Fatal(err)
//			}
//			benchData = d
//			putByteBuffer(buf)
//			ReplyPool.ReleaseConnectReply(r)
//			res.ReturnToVTPool()
//		}
//	})
//}

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

//func BenchmarkReplyMarshalJSONConnect(b *testing.B) {
//	for i := 0; i < b.N; i++ {
//		res := ConnectResultFromVTPool()
//		res.Client = "clientclientclientclientclientclientclientclientclientclient"
//		res.Version = "0.0.0"
//		res.Ping = 25
//		res.Pong = true
//		r := ReplyPool.AcquireConnectReply(res)
//		d, err := marshalJSONConnect(r)
//		if err != nil {
//			b.Fatal(err)
//		}
//		benchData = d
//		ReplyPool.ReleaseConnectReply(r)
//		res.ReturnToVTPool()
//	}
//	b.ReportAllocs()
//}

//func BenchmarkReplyMarshalJSONConnectNoCopy(b *testing.B) {
//	for i := 0; i < b.N; i++ {
//		res := ConnectResultFromVTPool()
//		res.Client = "clientclientclientclientclientclientclientclientclientclient"
//		res.Version = "0.0.0"
//		res.Ping = 25
//		res.Pong = true
//		r := ReplyPool.AcquireConnectReply(res)
//		buf := getByteBuffer(r.SizeVT())
//		d, err := marshalJSONConnectNoCopy(r, buf.B)
//		if err != nil {
//			b.Fatal(err)
//		}
//		benchData = d
//		putByteBuffer(buf)
//		ReplyPool.ReleaseConnectReply(r)
//		res.ReturnToVTPool()
//	}
//	b.ReportAllocs()
//}

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

//func BenchmarkReplyMarshalJSONConnectParallel(b *testing.B) {
//	b.ReportAllocs()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			res := ConnectResultFromVTPool()
//			res.Client = "clientclientclientclientclientclientclientclientclientclient"
//			res.Version = "0.0.0"
//			res.Ping = 25
//			res.Pong = true
//			r := ReplyPool.AcquireConnectReply(res)
//			d, err := marshalJSONConnect(r)
//			if err != nil {
//				b.Fatal(err)
//			}
//			benchData = d
//			ReplyPool.ReleaseConnectReply(r)
//			res.ReturnToVTPool()
//		}
//	})
//}

//func BenchmarkReplyMarshalJSONConnectNoCopyParallel(b *testing.B) {
//	b.ReportAllocs()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			res := ConnectResultFromVTPool()
//			res.Client = "clientclientclientclientclientclientclientclientclientclient"
//			res.Version = "0.0.0"
//			res.Ping = 25
//			res.Pong = true
//			r := ReplyPool.AcquireConnectReply(res)
//			buf := getByteBuffer(r.SizeVT())
//			d, err := marshalJSONConnectNoCopy(r, buf.B)
//			if err != nil {
//				b.Fatal(err)
//			}
//			benchData = d
//			putByteBuffer(buf)
//			ReplyPool.ReleaseConnectReply(r)
//			res.ReturnToVTPool()
//		}
//	})
//}

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

func buildJSONMultiFrame(b *testing.B, n int) []byte {
	b.Helper()
	encoder := NewJSONCommandEncoder()
	var out []byte
	for i := 0; i < n; i++ {
		cmd := &Command{
			Id: uint32(i + 1),
			Publish: &PublishRequest{
				Channel: "test",
				Data:    preparedPayload,
			},
		}
		data, err := encoder.Encode(cmd)
		if err != nil {
			b.Fatal(err)
		}
		out = append(out, data...)
		if i < n-1 {
			out = append(out, '\n')
		}
	}
	return out
}

func buildProtobufMultiFrame(b *testing.B, n int) []byte {
	b.Helper()
	encoder := NewProtobufCommandEncoder()
	var out []byte
	for i := 0; i < n; i++ {
		cmd := &Command{
			Id: uint32(i + 1),
			Publish: &PublishRequest{
				Channel: "test",
				Data:    preparedPayload,
			},
		}
		data, err := encoder.Encode(cmd)
		if err != nil {
			b.Fatal(err)
		}
		out = append(out, data...)
	}
	return out
}

func benchDecodeJSONMulti(b *testing.B, n int) {
	data := buildJSONMultiFrame(b, n)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		decoder := GetCommandDecoder(TypeJSON, data)
		for {
			cmd, err := decoder.Decode()
			if cmd != nil {
				benchConnectRequest = nil // sink
				_ = cmd
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
		PutCommandDecoder(TypeJSON, decoder)
	}
}

func benchDecodeProtobufMulti(b *testing.B, n int) {
	data := buildProtobufMultiFrame(b, n)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		decoder := GetCommandDecoder(TypeProtobuf, data)
		for {
			cmd, err := decoder.Decode()
			if cmd != nil {
				_ = cmd
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
		PutCommandDecoder(TypeProtobuf, decoder)
	}
}

func benchEncodeProtobufCommand(b *testing.B, n int) {
	cmd := &Command{
		Id: 1,
		Publish: &PublishRequest{
			Channel: "test",
			Data:    preparedPayload,
		},
	}
	enc := NewProtobufCommandEncoder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < n; j++ {
			d, err := enc.Encode(cmd)
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
		}
	}
}

func benchEncodeJSONCommand(b *testing.B, n int) {
	cmd := &Command{
		Id: 1,
		Publish: &PublishRequest{
			Channel: "test",
			Data:    preparedPayload,
		},
	}
	enc := NewJSONCommandEncoder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < n; j++ {
			d, err := enc.Encode(cmd)
			if err != nil {
				b.Fatal(err)
			}
			benchData = d
		}
	}
}

func benchProtobufDataEncoder(b *testing.B, n int) {
	frame := make([]byte, 280)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		enc := GetDataEncoder(TypeProtobuf)
		for j := 0; j < n; j++ {
			_ = enc.Encode(frame)
		}
		benchData = enc.Finish()
		PutDataEncoder(TypeProtobuf, enc)
	}
}

func benchJSONDataEncoder(b *testing.B, n int) {
	frame := make([]byte, 280)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		enc := GetDataEncoder(TypeJSON)
		for j := 0; j < n; j++ {
			_ = enc.Encode(frame)
		}
		benchData = enc.Finish()
		PutDataEncoder(TypeJSON, enc)
	}
}

func BenchmarkReplyEncodeProtobufOnly(b *testing.B) {
	r := &Reply{Push: &Push{Channel: "test", Pub: &Publication{Data: preparedPayload}}}
	enc := NewProtobufReplyEncoder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d, err := enc.Encode(r)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
	}
}

func BenchmarkReplyEncodeJSONOnly(b *testing.B) {
	r := &Reply{Push: &Push{Channel: "test", Pub: &Publication{Data: preparedPayload}}}
	enc := NewJSONReplyEncoder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d, err := enc.Encode(r)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
	}
}

func BenchmarkPushEncodeProtobufOnly(b *testing.B) {
	p := &Push{Channel: "test", Pub: &Publication{Data: preparedPayload}}
	enc := NewProtobufPushEncoder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d, err := enc.Encode(p)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
	}
}

func BenchmarkPushEncodeJSONOnly(b *testing.B) {
	p := &Push{Channel: "test", Pub: &Publication{Data: preparedPayload}}
	enc := NewJSONPushEncoder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d, err := enc.Encode(p)
		if err != nil {
			b.Fatal(err)
		}
		benchData = d
	}
}

func BenchmarkEncodeProtobufCommand1(b *testing.B)  { benchEncodeProtobufCommand(b, 1) }
func BenchmarkEncodeProtobufCommand64(b *testing.B) { benchEncodeProtobufCommand(b, 64) }
func BenchmarkEncodeJSONCommand1(b *testing.B)      { benchEncodeJSONCommand(b, 1) }
func BenchmarkEncodeJSONCommand64(b *testing.B)     { benchEncodeJSONCommand(b, 64) }
func BenchmarkProtobufDataEncoder8(b *testing.B)    { benchProtobufDataEncoder(b, 8) }
func BenchmarkProtobufDataEncoder64(b *testing.B)   { benchProtobufDataEncoder(b, 64) }
func BenchmarkJSONDataEncoder8(b *testing.B)        { benchJSONDataEncoder(b, 8) }
func BenchmarkJSONDataEncoder64(b *testing.B)       { benchJSONDataEncoder(b, 64) }

func BenchmarkCommandJSONUnmarshalMulti1(b *testing.B)   { benchDecodeJSONMulti(b, 1) }
func BenchmarkCommandJSONUnmarshalMulti8(b *testing.B)   { benchDecodeJSONMulti(b, 8) }
func BenchmarkCommandJSONUnmarshalMulti64(b *testing.B)  { benchDecodeJSONMulti(b, 64) }
func BenchmarkCommandJSONUnmarshalMulti256(b *testing.B) { benchDecodeJSONMulti(b, 256) }

func BenchmarkCommandProtobufUnmarshalMulti1(b *testing.B)   { benchDecodeProtobufMulti(b, 1) }
func BenchmarkCommandProtobufUnmarshalMulti8(b *testing.B)   { benchDecodeProtobufMulti(b, 8) }
func BenchmarkCommandProtobufUnmarshalMulti64(b *testing.B)  { benchDecodeProtobufMulti(b, 64) }
func BenchmarkCommandProtobufUnmarshalMulti256(b *testing.B) { benchDecodeProtobufMulti(b, 256) }
