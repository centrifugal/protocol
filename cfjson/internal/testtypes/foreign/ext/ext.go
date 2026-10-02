// Package ext plays the part of a package which is not ours to change: a
// dependency, or code shared with another project. It knows nothing about
// cfjson.
package ext

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// Audience is a string or a list of strings in JSON.
type Audience []string

// UnmarshalJSON accepts both forms.
func (a *Audience) UnmarshalJSON(data []byte) error {
	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		*a = Audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return errors.New("ext: audience must be a string or a list of strings")
	}
	*a = many
	return nil
}

// MarshalJSON writes a single value as a string.
func (a Audience) MarshalJSON() ([]byte, error) {
	if len(a) == 1 {
		return json.Marshal(a[0])
	}
	return json.Marshal([]string(a))
}

// NumericDate is a number of seconds in JSON, whole or not.
type NumericDate struct {
	time.Time
}

// UnmarshalJSON reads a number of seconds.
func (d *NumericDate) UnmarshalJSON(data []byte) error {
	seconds, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return errors.New("ext: a date must be a number")
	}
	d.Time = time.Unix(0, int64(seconds*float64(time.Second))).UTC()
	return nil
}

// MarshalJSON writes whole seconds.
func (d NumericDate) MarshalJSON() ([]byte, error) {
	return strconv.AppendInt(nil, d.Unix(), 10), nil
}

// RegisteredClaims are claims every token has, the way JWT libraries
// declare them.
type RegisteredClaims struct {
	Issuer    string       `json:"iss,omitempty"`
	Subject   string       `json:"sub,omitempty"`
	Audience  Audience     `json:"aud,omitempty"`
	ExpiresAt *NumericDate `json:"exp,omitempty"`
	NotBefore *NumericDate `json:"nbf,omitempty"`
	IssuedAt  *NumericDate `json:"iat,omitempty"`
	ID        string       `json:"jti,omitempty"`
}

// Capability is a plain struct of another package.
type Capability struct {
	Channels []string `json:"channels,omitempty"`
	Match    Match    `json:"match,omitempty"`
	Allow    []string `json:"allow,omitempty"`
	Limits   *Limits  `json:"limits,omitempty"`

	hidden int
}

// Match is a named string type of another package.
type Match string

// Limits is a struct a struct of another package refers to.
type Limits struct {
	Rate  float64 `json:"rate,omitempty"`
	Burst int     `json:"burst,omitempty"`
}

// Hidden returns the unexported field.
func (c *Capability) Hidden() int { return c.hidden }
