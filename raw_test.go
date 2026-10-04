package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Raw is how payloads are held: an already encoded JSON value for JSON, any
// bytes for Protobuf. Its JSON methods are what encoding/json uses for it.

func TestRaw_MarshalJSON(t *testing.T) {
	// Like json.RawMessage.
	std, err := json.Marshal(struct{ Data *json.RawMessage }{Data: &json.RawMessage{'{', '}'}})
	require.NoError(t, err)
	raw, err := json.Marshal(struct{ Data *Raw }{Data: &Raw{'{', '}'}})
	require.NoError(t, err)
	require.Equal(t, string(std), string(raw))

	// Nothing is null, as cfjson.AppendRaw writes it.
	for _, raw := range []Raw{nil, {}, Raw("\n\n")} {
		data, nullErr := raw.MarshalJSON()
		require.NoError(t, nullErr)
		require.Equal(t, "null", string(data))
	}

	// Newlines are dropped: they delimit messages in a frame.
	data, err := Raw("{\n  \"key\": \"value\"\n}").MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, `{  "key": "value"}`, string(data))
}

func TestRaw_UnmarshalJSON(t *testing.T) {
	var nilRaw *Raw
	require.Error(t, nilRaw.UnmarshalJSON([]byte(`{}`)))

	// A Raw holds a copy of what it was decoded from.
	var r Raw
	data := []byte(`{"key": "value"}`)
	require.NoError(t, r.UnmarshalJSON(data))
	data[0] = 'X'
	require.Equal(t, `{"key": "value"}`, string(r))
}
