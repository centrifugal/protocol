package foreign

// The instructions for the generator live here, in a file of their own, and
// what it writes goes to another one: claims.go is not touched. Only the
// types which are decoded and encoded directly are named, the generator
// finds what they are made of.

//go:generate go run ../../../cmd/cfjson -types ConnectClaims,SubscribeClaims,Depth,Tie,TagWins,DeepTie -out claims_cfjson.go claims.go embedded.go
