package protocol

import "testing"

func BenchmarkPubPushMarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pub := Publication{
			Data: nil,
		}
		data, err := pub.Marshal()
		if err != nil {
			b.Fail()
		}
		push := Push{
			Type:    PushTypePublication,
			Channel: "test",
			Data:    data,
		}
		data, err = push.Marshal()
		if err != nil {
			b.Fail()
		}
		cmd := Reply{
			ID:     1,
			Result: data,
		}
		_, err = cmd.Marshal()
		if err != nil {
			b.Fail()
		}
	}
	b.ReportAllocs()
}
