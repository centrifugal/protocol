package protocol

import (
	"io"
	"testing"

	"encoding/json"
)

func benchPayload() []byte {
	size := 256
	var p []byte
	for i := 0; i < size; i++ {
		p = append(p, 'i')
	}
	return []byte(`{"input": "` + string(p) + `"}`)
}

var preparedPayload = benchPayload()

func marshalProtobuf() ([]byte, error) {
	pushEncoder := NewProtobufPushEncoder()
	pub := &Publication{
		Data: preparedPayload,
	}
	data, err := pushEncoder.EncodePublication(pub)
	if err != nil {
		return nil, err
	}
	push := &Push{
		Type:    Push_PUBLICATION,
		Channel: "test",
		Data:    data,
	}
	data, err = pushEncoder.Encode(push)
	if err != nil {
		return nil, err
	}
	r := &Reply{
		Id:     1,
		Result: data,
	}
	encoder := NewProtobufReplyEncoder()
	data, _ = encoder.Encode(r)
	return data, nil
}

func marshalJSON() ([]byte, error) {
	pushEncoder := NewJSONPushEncoder()
	pub := &Publication{
		Data: preparedPayload,
	}
	data, err := pushEncoder.EncodePublication(pub)
	if err != nil {
		return nil, err
	}
	push := &Push{
		Type:    Push_PUBLICATION,
		Channel: "test",
		Data:    data,
	}
	data, err = pushEncoder.Encode(push)
	if err != nil {
		return nil, err
	}
	r := &Reply{
		Id:     1,
		Result: data,
	}
	encoder := NewJSONReplyEncoder()
	return encoder.Encode(r)
}

func BenchmarkReplyProtobufMarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := marshalProtobuf()
		if err != nil {
			b.Fail()
		}
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufMarshalParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := marshalProtobuf()
			if err != nil {
				b.Fail()
			}
		}
	})
}

func BenchmarkReplyProtobufUnmarshal(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := params.Marshal()
	cmd := &Command{
		Id:     1,
		Method: Command_CONNECT,
		Params: data,
	}
	encoder := NewProtobufCommandEncoder()
	data, _ = encoder.Encode(cmd)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unmarshalProtobuf(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyProtobufUnmarshalParallel(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := params.Marshal()
	cmd := &Command{
		Id:     1,
		Method: Command_CONNECT,
		Params: data,
	}
	encoder := NewProtobufCommandEncoder()
	data, _ = encoder.Encode(cmd)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			unmarshalProtobuf(b, data)
		}
	})
	b.ReportAllocs()
}

func unmarshalProtobuf(b *testing.B, data []byte) {
	decoder := GetCommandDecoder(TypeProtobuf, data)
	defer PutCommandDecoder(TypeProtobuf, decoder)
	cmd, err := decoder.Decode()
	if err != nil && err != io.EOF {
		b.Fatal(err)
	}
	if cmd == nil {
		b.Fatal("nil command")
	}
	paramsDecoder := NewProtobufParamsDecoder()
	req, err := paramsDecoder.DecodeConnect(cmd.Params)
	if err != nil {
		b.Fatal()
	}
	if req.Token != "token" {
		b.Fatal()
	}
}

func BenchmarkReplyJSONMarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := marshalJSON()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONMarshalParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := marshalJSON()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkReplyJSONUnmarshal(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := json.Marshal(params)
	cmd := &Command{
		Id:     1,
		Method: Command_CONNECT,
		Params: data,
	}
	encoder := NewJSONCommandEncoder()
	data, err := encoder.Encode(cmd)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unmarshalJSON(b, data)
	}
	b.ReportAllocs()
}

func BenchmarkReplyJSONUnmarshalParallel(b *testing.B) {
	params := &ConnectRequest{
		Token: "token",
	}
	data, _ := json.Marshal(params)
	cmd := &Command{
		Id:     1,
		Method: Command_CONNECT,
		Params: data,
	}
	encoder := NewJSONCommandEncoder()
	data, err := encoder.Encode(cmd)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			unmarshalJSON(b, data)
		}
	})
	b.ReportAllocs()

}

func unmarshalJSON(b *testing.B, data []byte) {
	decoder := GetCommandDecoder(TypeJSON, data)
	defer PutCommandDecoder(TypeJSON, decoder)
	cmd, err := decoder.Decode()
	if (err != nil && err != io.EOF) || cmd == nil {
		b.Fatal(err)
	}
	paramsDecoder := NewJSONParamsDecoder()
	req, err := paramsDecoder.DecodeConnect(cmd.Params)
	if err != nil {
		b.Fatal()
	}
	if req.Token != "token" {
		b.Fatal()
	}
}
