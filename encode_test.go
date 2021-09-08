package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	data := []byte(`{
  "num": "1"
}
`)
	pushEncoder := NewJSONPushEncoder()
	pub := &Publication{
		Data: data,
	}
	res, err := pushEncoder.EncodePublication(pub)
	require.NoError(t, err)

	require.Len(t, strings.Split(string(res), "\n"), 1)
}

func TestEncodeStd(t *testing.T) {
	data := []byte(`{
  "num": "1"
}
`)
	pub := &Publication{
		Data: data,
	}

	res, err := json.Marshal(pub)
	require.NoError(t, err)

	require.Len(t, strings.Split(string(res), "\n"), 1)
}
