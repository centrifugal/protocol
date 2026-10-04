// Package testtypes has structs covering everything the cfprotobuf generator
// supports, written the way protoc-gen-go writes them. The tests of the
// generated code live here too.
package testtypes

//go:generate go run ../../cmd/cfprotobuf -out types_cfprotobuf.go types.go

// Raw is a named bytes type, like the one protocol uses for payloads.
type Raw []byte

// Name is a named string type.
type Name string

// Level is a named integer type, like an enum.
type Level int32

// Empty has no fields.
type Empty struct {
	unknownFields []byte
}

// Leaf is a small message used as a nested one.
type Leaf struct {
	ID   uint32 `protobuf:"varint,1,opt,name=id,proto3"`
	Text string `protobuf:"bytes,2,opt,name=text,proto3"`

	unknownFields []byte
}

// All has a field of every supported kind. Field numbers are not in order
// and some need a tag of two or three bytes.
type All struct {
	String   string             `protobuf:"bytes,1,opt,name=string,proto3"`
	Bytes    []byte             `protobuf:"bytes,2,opt,name=bytes,proto3"`
	Raw      Raw                `protobuf:"bytes,3,opt,name=raw,proto3"`
	Bool     bool               `protobuf:"varint,4,opt,name=bool,proto3"`
	Int32    int32              `protobuf:"varint,5,opt,name=int32,proto3"`
	Int64    int64              `protobuf:"varint,6,opt,name=int64,proto3"`
	Uint32   uint32             `protobuf:"varint,7,opt,name=uint32,proto3"`
	Uint64   uint64             `protobuf:"varint,8,opt,name=uint64,proto3"`
	Sint64   int64              `protobuf:"zigzag64,9,opt,name=sint64,proto3"`
	Double   float64            `protobuf:"fixed64,10,opt,name=double,proto3"`
	Float    float32            `protobuf:"fixed32,11,opt,name=float,proto3"`
	Leaf     *Leaf              `protobuf:"bytes,12,opt,name=leaf,proto3"`
	Self     *All               `protobuf:"bytes,13,opt,name=self,proto3"`
	Leaves   []*Leaf            `protobuf:"bytes,14,rep,name=leaves,proto3"`
	Strings  []string           `protobuf:"bytes,15,rep,name=strings,proto3"`
	StrMap   map[string]string  `protobuf:"bytes,16,rep,name=str_map,json=strMap,proto3" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	LeafMap  map[string]*Leaf   `protobuf:"bytes,17,rep,name=leaf_map,json=leafMap,proto3" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	FloatMap map[string]float64 `protobuf:"bytes,18,rep,name=float_map,json=floatMap,proto3" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"fixed64,2,opt,name=value"`
	IntMap   map[string]int64   `protobuf:"bytes,19,rep,name=int_map,json=intMap,proto3" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"varint,2,opt,name=value"`
	BoolMap  map[Name]bool      `protobuf:"bytes,20,rep,name=bool_map,json=boolMap,proto3" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"varint,2,opt,name=value"`
	Name     Name               `protobuf:"bytes,21,opt,name=name,proto3"`
	Level    Level              `protobuf:"varint,22,opt,name=level,proto3"`
	Far      string             `protobuf:"bytes,2047,opt,name=far,proto3"`
	Farther  uint64             `protobuf:"varint,300000,opt,name=farther,proto3"`
	Farthest *Leaf              `protobuf:"bytes,536870911,opt,name=farthest,proto3"`

	// Not a part of the message.
	Skipped string
	hidden  int

	unknownFields []byte
}

// NoUnknown is a message without a place for unknown fields: they are
// dropped.
type NoUnknown struct {
	ID uint32 `protobuf:"varint,1,opt,name=id,proto3"`
}

// Ping and Pong refer to each other.
type Ping struct {
	Pong *Pong `protobuf:"bytes,1,opt,name=pong,proto3"`
}

// Pong is the other half of Ping.
type Pong struct {
	Pings map[string]*Ping `protobuf:"bytes,1,rep,name=pings,proto3" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
}

// Unknown returns the fields of a decoded message which All does not have.
func (m *All) Unknown() []byte { return m.unknownFields }

// Hidden returns the unexported field, which generated code must ignore.
func (m *All) Hidden() int { return m.hidden }
