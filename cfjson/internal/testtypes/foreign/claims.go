// Package foreign has structs which use types of other packages and embed
// structs, generated for without a change to this file: it stands for a file
// which is maintained elsewhere.
package foreign

import (
	"encoding/json"
	"time"

	"github.com/centrifugal/protocol/cfjson/internal/testtypes/foreign/ext"
)

// SubscribeOptions is embedded in SubscribeClaims and used by value in
// ConnectClaims.
type SubscribeOptions struct {
	Info     json.RawMessage `json:"info,omitempty"`
	Presence bool            `json:"presence,omitempty"`
	ExpireAt *int64          `json:"expire_at,omitempty"`
}

// Extra is embedded by pointer.
type Extra struct {
	Note  string `json:"note,omitempty"`
	Shard int    `json:"shard,omitempty"`
}

// ConnectClaims has the shape of the claims of a connection token.
type ConnectClaims struct {
	ExpireAt *int64                      `json:"expire_at,omitempty"`
	Info     json.RawMessage             `json:"info,omitempty"`
	Channels []string                    `json:"channels,omitempty"`
	Subs     map[string]SubscribeOptions `json:"subs,omitempty"`
	Meta     json.RawMessage             `json:"meta,omitempty"`
	Caps     []*ext.Capability           `json:"caps,omitempty"`
	Cap      ext.Capability              `json:"cap"`
	Labels   map[string]string           `json:"labels,omitempty"`
	Channel  string                      `json:"channel,omitempty"`
	Seen     time.Time                   `json:"seen"`
	Until    *time.Time                  `json:"until,omitempty"`
	ext.RegisteredClaims
}

// SubscribeClaims embeds a struct of another package, one of this package,
// and one by pointer. Its own Subject hides the one of RegisteredClaims.
type SubscribeClaims struct {
	ext.RegisteredClaims
	SubscribeOptions
	*Extra
	Channel string `json:"channel,omitempty"`
	Subject string `json:"sub,omitempty"`
}
