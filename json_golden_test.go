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

// jsonMessage is implemented by every generated type.
type jsonMessage interface {
	cfjson.Appender
	cfjson.Decoder
}

// jsonMessages has a constructor for every message type of the protocol.
var jsonMessages = map[string]func() jsonMessage{
	"Error":                func() jsonMessage { return new(Error) },
	"EmulationRequest":     func() jsonMessage { return new(EmulationRequest) },
	"Command":              func() jsonMessage { return new(Command) },
	"Reply":                func() jsonMessage { return new(Reply) },
	"Push":                 func() jsonMessage { return new(Push) },
	"Dictionary":           func() jsonMessage { return new(Dictionary) },
	"ClientInfo":           func() jsonMessage { return new(ClientInfo) },
	"Publication":          func() jsonMessage { return new(Publication) },
	"Join":                 func() jsonMessage { return new(Join) },
	"Leave":                func() jsonMessage { return new(Leave) },
	"Unsubscribe":          func() jsonMessage { return new(Unsubscribe) },
	"Subscribe":            func() jsonMessage { return new(Subscribe) },
	"Message":              func() jsonMessage { return new(Message) },
	"Connect":              func() jsonMessage { return new(Connect) },
	"Disconnect":           func() jsonMessage { return new(Disconnect) },
	"Refresh":              func() jsonMessage { return new(Refresh) },
	"ConnectRequest":       func() jsonMessage { return new(ConnectRequest) },
	"ConnectResult":        func() jsonMessage { return new(ConnectResult) },
	"RefreshRequest":       func() jsonMessage { return new(RefreshRequest) },
	"RefreshResult":        func() jsonMessage { return new(RefreshResult) },
	"SubscribeRequest":     func() jsonMessage { return new(SubscribeRequest) },
	"SubscribeResult":      func() jsonMessage { return new(SubscribeResult) },
	"KeyedItem":            func() jsonMessage { return new(KeyedItem) },
	"TrackBatch":           func() jsonMessage { return new(TrackBatch) },
	"SubRefreshRequest":    func() jsonMessage { return new(SubRefreshRequest) },
	"SubRefreshResult":     func() jsonMessage { return new(SubRefreshResult) },
	"UnsubscribeRequest":   func() jsonMessage { return new(UnsubscribeRequest) },
	"UnsubscribeResult":    func() jsonMessage { return new(UnsubscribeResult) },
	"PublishRequest":       func() jsonMessage { return new(PublishRequest) },
	"PublishResult":        func() jsonMessage { return new(PublishResult) },
	"PresenceRequest":      func() jsonMessage { return new(PresenceRequest) },
	"PresenceResult":       func() jsonMessage { return new(PresenceResult) },
	"PresenceStatsRequest": func() jsonMessage { return new(PresenceStatsRequest) },
	"PresenceStatsResult":  func() jsonMessage { return new(PresenceStatsResult) },
	"StreamPosition":       func() jsonMessage { return new(StreamPosition) },
	"HistoryRequest":       func() jsonMessage { return new(HistoryRequest) },
	"HistoryResult":        func() jsonMessage { return new(HistoryResult) },
	"PingRequest":          func() jsonMessage { return new(PingRequest) },
	"PingResult":           func() jsonMessage { return new(PingResult) },
	"RPCRequest":           func() jsonMessage { return new(RPCRequest) },
	"RPCResult":            func() jsonMessage { return new(RPCResult) },
	"SendRequest":          func() jsonMessage { return new(SendRequest) },
	"FilterNode":           func() jsonMessage { return new(FilterNode) },
}

// Every struct of client.pb.go must be in jsonMessages, so that it is covered
// by the tests below.
func TestJSONMessagesComplete(t *testing.T) {
	src, err := os.ReadFile("client.pb.go")
	require.NoError(t, err)
	count := 0
	for _, line := range strings.Split(string(src), "\n") {
		if name, ok := strings.CutPrefix(line, "type "); ok && strings.HasSuffix(name, " struct {") {
			name = strings.TrimSuffix(name, " struct {")
			require.Contains(t, jsonMessages, name)
			count++
		}
	}
	require.Equal(t, len(jsonMessages), count)
}

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
		newMessage, ok := jsonMessages[name]
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
