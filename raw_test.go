package protocol

import (
	"encoding/json"
	"testing"
)

type DataRawMessage struct {
	Data *json.RawMessage
}

type DataRaw struct {
	Data *Raw
}

func TestRaw(t *testing.T) {
	data1 := json.RawMessage(`{"key": "value"}`)
	stdJsonData1, err := json.Marshal(DataRawMessage{
		Data: &data1,
	})
	if err != nil {
		t.Fatalf("%v", err)
	}

	data2 := Raw(`{"key": "value"}`)
	stdJsonData2, err := json.Marshal(DataRaw{
		Data: &data2,
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if string(stdJsonData1) != string(stdJsonData2) {
		t.Fatalf("no match: %v", err)
	}
}

func TestRaw_MarshalJSON_Nil(t *testing.T) {
	var r Raw
	data, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if string(data) != "null" {
		t.Fatalf("expected null, got %s", data)
	}
}

func TestRaw_MarshalJSON_StripsNewlines(t *testing.T) {
	r := Raw("{\n  \"key\": \"value\"\n}")
	data, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if string(data) != `{  "key": "value"}` {
		t.Fatalf("unexpected result: %s", data)
	}
}

func TestRaw_UnmarshalJSON_NilPointer(t *testing.T) {
	var r *Raw
	err := r.UnmarshalJSON([]byte(`{}`))
	if err == nil {
		t.Fatal("expected error on nil pointer")
	}
}

func TestRaw_UnmarshalJSON_CopiesData(t *testing.T) {
	var r Raw
	data := []byte(`{"key": "value"}`)
	if err := r.UnmarshalJSON(data); err != nil {
		t.Fatalf("%v", err)
	}
	data[0] = 'X'
	if r[0] == 'X' {
		t.Fatal("Raw should hold a copy, not alias the input slice")
	}
}
