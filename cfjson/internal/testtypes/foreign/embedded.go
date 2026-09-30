package foreign

// Structs which get the same JSON name from more than one embedded struct.
// encoding/json has a rule for which of the fields is the field, and
// generated code must follow it. The names which come twice are the point,
// hence the nolint comments.

// Deep is embedded in Middle.
type Deep struct {
	X string `json:"x"`
	Y string `json:"y"`
}

// Middle puts the fields of Deep one level deeper.
type Middle struct{ Deep }

// Shallow has the x Deep has.
type Shallow struct {
	X string `json:"x"`
}

// Depth has x twice: the one of Shallow is embedded less deep and wins,
// although the one of Deep is declared first.
type Depth struct {
	Middle
	Shallow
}

// One has z.
type One struct {
	Z string `json:"z"`
}

// Two has z as well.
type Two struct {
	Z string `json:"z"`
	W int    `json:"w"`
}

// Tie has z twice at the same depth: it has no field z then.
type Tie struct {
	One
	Two //nolint:govet // structtag: z is there twice on purpose.
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

// Level3 has z.
type Level3 struct {
	Z string `json:"z"`
}

// Level2 puts it one level deeper.
type Level2 struct{ Level3 }

// Level1 puts it one more level deeper.
type Level1 struct{ Level2 }

// DeepTie has z twice at one depth and once deeper. The two cancel each
// other, which does not make the deeper one a field.
type DeepTie struct {
	Tie    //nolint:govet // structtag: z is there once more on purpose.
	Level1 //nolint:govet // structtag: z is there once more on purpose.
}
