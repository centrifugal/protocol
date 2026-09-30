package foreign

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/centrifugal/protocol/cfjson"
	"github.com/centrifugal/protocol/cfjson/gen"
	"github.com/centrifugal/protocol/cfjson/internal/testtypes/foreign/ext"
)

func TestGeneratedCodeIsUpToDate(t *testing.T) {
	want, err := gen.Generate(gen.Config{
		Files: []string{"claims.go", "embedded.go"},
		Types: []string{"ConnectClaims", "SubscribeClaims", "Depth", "Tie", "TagWins", "DeepTie"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("claims_cfjson.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("claims_cfjson.go is out of date, run go generate")
	}
}

// The file with the structs must have nothing in it which is there for the
// generator: that is the point of this package.
func TestSourceIsUntouched(t *testing.T) {
	src, err := os.ReadFile("claims.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{`"github.com/centrifugal/protocol/cfjson"`, "cfjson.", "go:generate", "AppendJSON", "DecodeJSON"} {
		if strings.Contains(string(src), word) {
			t.Fatalf("claims.go mentions %s", word)
		}
	}
}

type value interface {
	cfjson.Appender
	cfjson.Decoder
}

func date(sec int64) *ext.NumericDate {
	return &ext.NumericDate{Time: time.Unix(sec, 0).UTC()}
}

func values() []value {
	expire := int64(1760003600)
	until := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	return []value{
		&ConnectClaims{},
		&ConnectClaims{
			ExpireAt: &expire,
			Info:     stdjson.RawMessage(`{"name":"Alex"}`),
			Channels: []string{"news", "chat:42"},
			Subs:     map[string]SubscribeOptions{"news": {Presence: true, Info: stdjson.RawMessage(`[1,2]`)}},
			Meta:     stdjson.RawMessage(`{"plan":"pro"}`),
			Caps: []*ext.Capability{
				{Channels: []string{"chat:*"}, Match: "wildcard", Allow: []string{"sub", "pub"}, Limits: &ext.Limits{Rate: 2.5, Burst: 10}},
				nil,
			},
			Cap:     ext.Capability{Match: "exact"},
			Labels:  map[string]string{"tenant": "acme"},
			Channel: "c",
			Seen:    time.Date(2026, 1, 2, 3, 4, 5, 600000000, time.UTC),
			Until:   &until,
			RegisteredClaims: ext.RegisteredClaims{
				Issuer: "app", Subject: "user-42", Audience: ext.Audience{"centrifugo"},
				ExpiresAt: date(1760000000), NotBefore: date(1759990000), IssuedAt: date(1759990000), ID: "jti-1",
			},
		},
		&ConnectClaims{RegisteredClaims: ext.RegisteredClaims{Audience: ext.Audience{"a", "b"}}},
		&SubscribeClaims{},
		&SubscribeClaims{
			RegisteredClaims: ext.RegisteredClaims{Issuer: "app", Subject: "hidden by the field of the struct itself", ID: "x", ExpiresAt: date(1)},
			SubscribeOptions: SubscribeOptions{Presence: true, ExpireAt: &expire, Info: stdjson.RawMessage(`"i"`)},
			Extra:            &Extra{Note: "n", Shard: 3},
			Channel:          "c",
			Subject:          "user-42",
		},
		&SubscribeClaims{Extra: &Extra{}},
		&SubscribeOptions{},
	}
}

func decode(data []byte, v value) error {
	return cfjson.Unmarshal(data, v, 0)
}

func zero(v value) value {
	return reflect.New(reflect.TypeOf(v).Elem()).Interface().(value)
}

// Generated code must agree with encoding/json on these structs, which use
// everything it has for types the generator cannot add methods to: a struct
// of another package by value, by pointer and embedded, types with JSON
// methods of their own, json.RawMessage, time.Time.
func TestMatchesEncodingJSON(t *testing.T) {
	for n, v := range values() {
		ours := v.AppendJSON(nil)
		std, err := stdjson.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		// Without a map of several keys the two write the same bytes.
		if !bytes.Equal(ours, std) {
			t.Errorf("value %d:\nours %s\n std %s", n, ours, std)
		}
		want := zero(v)
		if err := stdjson.Unmarshal(std, want); err != nil {
			t.Fatal(err)
		}
		got := zero(v)
		if err := decode(std, got); err != nil {
			t.Fatalf("value %d: %v: %s", n, err, std)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("value %d decoded from %s:\n got %+v\nwant %+v", n, std, got, want)
		}
	}
}

func TestDecodeForeign(t *testing.T) {
	// The types of the other package read their JSON themselves: a single
	// audience, a date with a fraction.
	var c ConnectClaims
	input := `{"aud":"one","exp":1760000000.5,"caps":[{"match":"m","limits":{"burst":3}}],"seen":"2026-01-02T03:04:05Z","sub":"u"}`
	if err := decode([]byte(input), &c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Audience, ext.Audience{"one"}) || c.ExpiresAt.UnixMilli() != 1760000000500 || c.Subject != "u" ||
		c.Caps[0].Match != "m" || c.Caps[0].Limits.Burst != 3 || c.Seen.Year() != 2026 {
		t.Fatalf("got %+v", c)
	}
	var want ConnectClaims
	if err := stdjson.Unmarshal([]byte(input), &want); err != nil || !reflect.DeepEqual(c, want) {
		t.Fatalf("encoding/json disagrees: %v", err)
	}

	// An embedded pointer is allocated when one of its fields comes, and
	// only then.
	var s SubscribeClaims
	if err := decode([]byte(`{"channel":"c","iss":"app"}`), &s); err != nil || s.Extra != nil || s.Issuer != "app" {
		t.Fatalf("got %+v, %v", s, err)
	}
	if err := decode([]byte(`{"shard":7}`), &s); err != nil || s.Extra == nil || s.Shard != 7 {
		t.Fatalf("got %+v, %v", s, err)
	}
	// The field of the struct wins over the one it hides.
	s = SubscribeClaims{}
	if err := decode([]byte(`{"sub":"mine"}`), &s); err != nil || s.Subject != "mine" || s.RegisteredClaims.Subject != "" {
		t.Fatalf("got %+v, %v", s, err)
	}

	// What a type of the other package refuses is an error here, and so is
	// a key which repeats, whichever struct its field comes from.
	for _, bad := range []string{
		`{"aud":5}`, `{"exp":"soon"}`, `{"seen":"yesterday"}`, `{"until":17}`, `{"caps":[{"limits":{"burst":"x"}}]}`,
		`{"iss":"a","iss":"b"}`, `{"caps":[{"match":"a","match":"b"}]}`, `{"cap":{"allow":[],"allow":[]}}`,
	} {
		var de *cfjson.DecodeError
		if err := decode([]byte(bad), new(ConnectClaims)); !errors.As(err, &de) {
			t.Errorf("%s: got %v, want a DecodeError", bad, err)
		}
	}
	for _, bad := range []string{`{"sub":"a","sub":"b"}`, `{"note":"a","note":"b"}`, `{"presence":true,"presence":true}`} {
		if err := decode([]byte(bad), new(SubscribeClaims)); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

// A MarshalJSON which fails, or returns something which is not JSON, must not
// damage what is around it.
type broken struct{ out string }

func (b broken) MarshalJSON() ([]byte, error) {
	if b.out == "" {
		return nil, errors.New("no")
	}
	return []byte(b.out), nil
}

func TestAppendMarshaler(t *testing.T) {
	for out, want := range map[string]string{"": "[null]", "{": "[null]", "1 2": "[null]", `{"a":1}`: `[{"a":1}]`, " true ": "[ true ]"} {
		got := append(cfjson.AppendMarshaler([]byte("["), broken{out}), ']')
		if string(got) != want {
			t.Errorf("MarshalJSON returning %q: got %s, want %s", out, got, want)
		}
	}
}

// Which of the fields with the same name is the field, see embedded.go.
func TestEmbeddedFieldsMatchEncodingJSON(t *testing.T) {
	depth := &Depth{Middle: Middle{Deep: Deep{X: "deep", Y: "y"}}, Shallow: Shallow{X: "shallow"}}
	tie := &Tie{One: One{Z: "one"}, Two: Two{Z: "two", W: 1}}
	tagWins := &TagWins{Untagged: Untagged{N: "untagged"}, Tagged: Tagged{V: "tagged"}}
	deepTie := &DeepTie{Tie: *tie, Level1: Level1{Level2: Level2{Level3: Level3{Z: "three"}}}}
	for _, v := range []value{depth, tie, tagWins, deepTie} {
		want, err := stdjson.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if got := v.AppendJSON(nil); !bytes.Equal(got, want) {
			t.Errorf("%T:\n cfjson %s\n    std %s", v, got, want)
		}
		input := []byte(`{"x":"X","y":"Y","z":"Z","w":7,"N":"n"}`)
		std, got := zero(v), zero(v)
		if err := stdjson.Unmarshal(input, std); err != nil {
			t.Fatal(err)
		}
		if err := decode(input, got); err != nil {
			t.Fatalf("%T: %v", v, err)
		}
		if !reflect.DeepEqual(std, got) {
			t.Errorf("%T decoded:\n cfjson %+v\n    std %+v", v, got, std)
		}
	}
	if got := string(depth.AppendJSON(nil)); got != `{"y":"y","x":"shallow"}` {
		t.Errorf("Depth: %s", got)
	}
	if got := string(deepTie.AppendJSON(nil)); got != `{"w":1}` {
		t.Errorf("DeepTie: %s", got)
	}
}
