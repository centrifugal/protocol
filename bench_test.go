package protocol

import (
	"encoding/json"
	"testing"
)

func BenchmarkReplyMarshalProtobuf(b *testing.B) {
	pub := Publication{
		Data: nil,
	}
	for i := 0; i < b.N; i++ {
		data, err := pub.Marshal()
		if err != nil {
			b.Fail()
		}
		push := Push{
			Type:    PushType_PUSH_TYPE_PUBLICATION,
			Channel: "test",
			Data:    data,
		}
		data, err = push.Marshal()
		if err != nil {
			b.Fail()
		}
		cmd := Reply{
			Id:     1,
			Result: data,
		}
		_, err = cmd.Marshal()
		if err != nil {
			b.Fail()
		}
	}
	b.ReportAllocs()
}

func BenchmarkReplyMarshalJSON(b *testing.B) {
	pub := Publication{
		Data: nil,
	}
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(pub)
		if err != nil {
			b.Fail()
		}
		push := Push{
			Type:    PushType_PUSH_TYPE_PUBLICATION,
			Channel: "test",
			Data:    data,
		}
		data, err = json.Marshal(push)
		if err != nil {
			b.Fail()
		}
		cmd := Reply{
			Id:     1,
			Result: data,
		}
		_, err = json.Marshal(cmd)
		if err != nil {
			b.Fail()
		}
	}
	b.ReportAllocs()
}
