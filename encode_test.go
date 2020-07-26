package protocol

import "testing"

func TestJSONEncoder(t *testing.T) {
	enc := NewJSONReplyEncoder()
	err := enc.Encode(&Reply{
		ID: 1,
	})
	if err != nil {
		t.Error(err)
	}
	data := enc.Finish()
	if string(data) != "{\"id\":1}" {
		t.Fail()
	}
	enc.Reset()
	err = enc.Encode(&Reply{
		ID: 2,
	})
	if err != nil {
		t.Error(err)
	}
	err = enc.Encode(&Reply{
		ID: 3,
	})
	if err != nil {
		t.Error(err)
	}
	data = enc.Finish()
	if string(data) != "{\"id\":2}\n{\"id\":3}" {
		t.Fail()
	}
}
