//go:build !js

package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	fastJSON "github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/require"
)

func TestEncodeEasyJson(t *testing.T) {
	data := []byte(`{
  "num": "1\n"

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
  "num": "1\n"

}
`)
	pub := &Publication{
		Data: data,
	}

	res, err := json.Marshal(pub)
	require.NoError(t, err)
	require.Len(t, strings.Split(string(res), "\n"), 1)
}

func TestEncodeFast(t *testing.T) {
	data := []byte(`{
  "num": "1\n"

}
`)
	pub := &Publication{
		Data: data,
	}

	res, err := fastJSON.Marshal(pub)
	require.NoError(t, err)
	require.Len(t, strings.Split(string(res), "\n"), 1)
}
