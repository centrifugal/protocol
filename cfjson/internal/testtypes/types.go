// Package testtypes has structs covering everything the cfjson generator
// supports, the tests of the generated code live here too.
package testtypes

//go:generate go run ../../cmd/cfjson -raw Raw -out types_cfjson.go types.go

// Raw is an already encoded JSON value.
type Raw []byte

// Level is a named integer type, like an enum.
type Level int32

// Name is a named string type.
type Name string

// Labels is a named map type.
type Labels map[string]string

// Names is a named slice type.
type Names []Name

// Empty has no fields at all.
type Empty struct{}

// Leaf is a small struct used as a nested value.
type Leaf struct {
	ID   uint32 `json:"id,omitempty"`
	Text string `json:"text,omitempty"`
}

// All has a field of every supported kind, with omitempty.
type All struct {
	String    string              `json:"string,omitempty"`
	Bool      bool                `json:"bool,omitempty"`
	Int       int                 `json:"int,omitempty"`
	Int8      int8                `json:"int8,omitempty"`
	Int16     int16               `json:"int16,omitempty"`
	Int32     int32               `json:"int32,omitempty"`
	Int64     int64               `json:"int64,omitempty"`
	Uint      uint                `json:"uint,omitempty"`
	Uint8     uint8               `json:"uint8,omitempty"`
	Uint16    uint16              `json:"uint16,omitempty"`
	Uint32    uint32              `json:"uint32,omitempty"`
	Uint64    uint64              `json:"uint64,omitempty"`
	Float32   float32             `json:"float32,omitempty"`
	Float64   float64             `json:"float64,omitempty"`
	Raw       Raw                 `json:"raw,omitempty"`
	Level     Level               `json:"level,omitempty"`
	Name      Name                `json:"name,omitempty"`
	Labels    Labels              `json:"labels,omitempty"`
	Names     Names               `json:"names,omitempty"`
	Leaf      Leaf                `json:"leaf,omitempty"`
	LeafPtr   *Leaf               `json:"leaf_ptr,omitempty"`
	Self      *All                `json:"self,omitempty"`
	StrPtr    *string             `json:"str_ptr,omitempty"`
	IntPtr    *int64              `json:"int_ptr,omitempty"`
	Strings   []string            `json:"strings,omitempty"`
	Ints      []int64             `json:"ints,omitempty"`
	Floats    []float64           `json:"floats,omitempty"`
	Bools     []bool              `json:"bools,omitempty"`
	Raws      []Raw               `json:"raws,omitempty"`
	Leaves    []Leaf              `json:"leaves,omitempty"`
	LeafPtrs  []*Leaf             `json:"leaf_ptrs,omitempty"`
	Matrix    [][]uint32          `json:"matrix,omitempty"`
	StrMap    map[string]string   `json:"str_map,omitempty"`
	FloatMap  map[string]float64  `json:"float_map,omitempty"`
	LeafMap   map[string]*Leaf    `json:"leaf_map,omitempty"`
	ListMap   map[string][]string `json:"list_map,omitempty"`
	NameMap   map[Name]Level      `json:"name_map,omitempty"`
	Skipped   string              `json:"-"`
	hidden    string
	MixedCase string `json:"mixedCase,omitempty"`
	Untagged  string
}

// Required has the same kinds without omitempty: every field is always
// written.
type Required struct {
	String   string            `json:"string"`
	Bool     bool              `json:"bool"`
	Int64    int64             `json:"int64"`
	Uint32   uint32            `json:"uint32"`
	Float64  float64           `json:"float64"`
	Raw      Raw               `json:"raw"`
	Leaf     Leaf              `json:"leaf"`
	LeafPtr  *Leaf             `json:"leaf_ptr"`
	StrPtr   *string           `json:"str_ptr"`
	Strings  []string          `json:"strings"`
	LeafPtrs []*Leaf           `json:"leaf_ptrs"`
	StrMap   map[string]string `json:"str_map"`
	LeafMap  map[string]*Leaf  `json:"leaf_map"`
}

// Mixed starts with optional fields followed by a required one.
type Mixed struct {
	A string `json:"a,omitempty"`
	B string `json:"b"`
	C string `json:"c,omitempty"`
}

// Hidden returns the unexported field, which generated code must ignore.
func (a *All) Hidden() string { return a.hidden }
