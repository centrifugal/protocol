// Package fold has types decoded with the -fold-keys option of the cfjson
// generator: keys are matched without regard to letter case when they are not
// the name of a field as written.
package fold

//go:generate go run ../../../cmd/cfjson -fold-keys -out types_cfjson.go types.go

// Lower has field names which are all in lower case.
type Lower struct {
	Channel string            `json:"channel,omitempty"`
	User    string            `json:"user,omitempty"`
	Info    *Lower            `json:"info,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
}

// Mixed has names with upper case letters in them.
type Mixed struct {
	NumClients int    `json:"numClients,omitempty"`
	ID         string `json:"ID,omitempty"`
	Untagged   string
}
