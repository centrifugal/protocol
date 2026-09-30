package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/centrifugal/protocol/cfjson"
)

// testdata/json_golden.txt holds messages encoded by the easyjson based
// encoders this package had up to v0.22, one per line. Client SDKs were
// written against that output, so the encoders must keep producing it byte
// for byte.
//
// The file is generated (and kept honest) by internal/cfjsoncmp, the module
// which still has the easyjson code. Here a message is first decoded with
// encoding/json, which shares nothing with the code under test, and then
// encoded again.
func TestJSONGolden(t *testing.T) {
	f, err := os.Open("testdata/json_golden.txt")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1<<20)
	lines := 0
	for scanner.Scan() {
		lines++
		name, golden, ok := strings.Cut(scanner.Text(), "\t")
		require.True(t, ok)
		newMessage, ok := messageTypes[name]
		require.True(t, ok, name)

		msg := newMessage()
		require.NoError(t, json.Unmarshal([]byte(golden), msg))
		require.Equal(t, golden, string(msg.AppendJSON(nil)), name)
		require.Equal(t, golden, string(encodeJSON(msg)), name)

		// The decoders must make the same message of it as encoding/json.
		for _, flags := range []cfjson.Flags{0, cfjson.ZeroCopy} {
			decoded := newMessage()
			data := []byte(golden)
			n := decoded.DecodeJSON(data, 0, flags)
			require.Equal(t, len(data), n, "%s: %s", name, golden)
			require.True(t, reflect.DeepEqual(msg, decoded), "%s: %s", name, golden)
			require.True(t, bytes.Equal(data, []byte(golden)), "decoding modified its input")
		}
	}
	require.NoError(t, scanner.Err())
	require.Greater(t, lines, 1000)
}
