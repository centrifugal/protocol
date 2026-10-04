package foreign

// Structs which get the same JSON name from more than one embedded struct.
// encoding/json has a rule for which of the fields is the field, and
// generated code must follow it. The fields have no tags where they do not
// need them: go vet takes a tag which comes twice for a mistake, and here it
// is the point.

// Deep is embedded in Middle.
type Deep struct {
	X string
	Y string
}

// Middle puts the fields of Deep one level deeper.
type Middle struct{ Deep }

// Shallow has the X Deep has.
type Shallow struct {
	X string
}

// Depth has X twice: the one of Shallow is embedded less deep and wins,
// although the one of Deep is declared first.
type Depth struct {
	Middle
	Shallow
}

// One has Z.
type One struct {
	Z string
}

// Two has Z as well.
type Two struct {
	Z string
	W int
}

// Tie has Z twice at the same depth: it has no field Z then.
type Tie struct {
	One
	Two
}

// Untagged has a field which is N by its name.
type Untagged struct{ N string }

// Tagged has a field which is N by its tag.
type Tagged struct {
	V string `json:"N"`
}

// TagWins has N twice at the same depth, and the one named by a tag wins.
type TagWins struct {
	Untagged
	Tagged
}

// Level3 has Z.
type Level3 struct {
	Z string
}

// Level2 puts it one level deeper.
type Level2 struct{ Level3 }

// Level1 puts it one more level deeper.
type Level1 struct{ Level2 }

// DeepTie has Z twice at one depth and once deeper. The two cancel each
// other, which does not make the deeper one a field.
type DeepTie struct {
	Tie
	Level1
}
