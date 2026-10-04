package fold

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/centrifugal/protocol/cfjson"
	"github.com/centrifugal/protocol/cfjson/gen"
)

func TestGeneratedCodeIsUpToDate(t *testing.T) {
	want, err := gen.Generate(gen.Config{Files: []string{"types.go"}, FoldKeys: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("types_cfjson.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("types_cfjson.go is out of date, run go generate")
	}
}

// With the option a key is matched the way encoding/json matches it.
func TestFoldKeysMatchesEncodingJSON(t *testing.T) {
	for _, input := range []string{
		`{"channel":"c","user":"u"}`,
		`{"Channel":"c","USER":"u"}`,
		`{"CHANNEL":"c","uSeR":"u","unknown":1,"UNKNOWN":2}`,
		`{"Info":{"Channel":"c","INFO":{"user":"u"}}}`,
		`{"Tags":{"Key":"v","key":"w"}}`,
		`{"channe":"c","channell":"c","":"c"}`,
	} {
		var got, want Lower
		if err := cfjson.Unmarshal([]byte(input), &got, 0); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if err := stdjson.Unmarshal([]byte(input), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %+v\nwant %+v", input, got, want)
		}
	}
	for _, input := range []string{
		`{"numClients":1,"ID":"a","Untagged":"u"}`,
		`{"numclients":1,"id":"a","untagged":"u"}`,
		`{"NUMCLIENTS":1,"Id":"a","UNTAGGED":"u"}`,
		`{"NumClients":1,"iD":"a","unTagged":"u","num_clients":5}`,
	} {
		var got, want Mixed
		if err := cfjson.Unmarshal([]byte(input), &got, 0); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if err := stdjson.Unmarshal([]byte(input), &want); err != nil {
			t.Fatal(err)
		}
		if got != want || got.NumClients != 1 {
			t.Errorf("%s:\n got %+v\nwant %+v", input, got, want)
		}
	}
}

// A key may still come once, however it is written: matching without regard
// to case must not bring back the ambiguity a repeated key is.
func TestFoldKeysRejectsRepeatedKeys(t *testing.T) {
	for _, input := range []string{
		`{"channel":"a","channel":"b"}`,
		`{"channel":"a","Channel":"b"}`,
		`{"CHANNEL":"a","channel":"b"}`,
		`{"Channel":"a","cHANNEL":"b"}`,
		`{"info":{"user":"a","USER":"b"}}`,
	} {
		var de *cfjson.DecodeError
		if err := cfjson.Unmarshal([]byte(input), new(Lower), 0); !errors.As(err, &de) || de.Syntax {
			t.Errorf("%s: got %v, want a DecodeError which is not a syntax error", input, err)
		}
	}
	for _, input := range []string{`{"numClients":1,"NumClients":2}`, `{"id":"a","ID":"b"}`, `{"untagged":"a","Untagged":"b"}`} {
		var de *cfjson.DecodeError
		if err := cfjson.Unmarshal([]byte(input), new(Mixed), 0); !errors.As(err, &de) || de.Syntax {
			t.Errorf("%s: got %v, want a DecodeError which is not a syntax error", input, err)
		}
	}
	// Keys of a map are data, not names: they are never folded.
	var l Lower
	if err := cfjson.Unmarshal([]byte(`{"tags":{"a":"1","A":"2"}}`), &l, 0); err != nil || len(l.Tags) != 2 {
		t.Fatalf("got %v, %v", l.Tags, err)
	}
}
