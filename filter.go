package protocol

import (
	"errors"
	"fmt"
	"slices"
)

// Node operations,
const (
	FilterOpLeaf = "" // leaf node
	FilterOpAnd  = "and"
	FilterOpOr   = "or"
	FilterOpNot  = "not"
)

// Leaf comparison operators.
const (
	FilterCompareEQ        = "eq"
	FilterCompareNotEQ     = "neq"
	FilterCompareIn        = "in"
	FilterCompareNotIn     = "nin"
	FilterCompareExists    = "ex"
	FilterCompareNotExists = "nex"
)

func (f *FilterNode) Match(tags map[string]string) (bool, error) {
	switch f.Op {
	case FilterOpLeaf: // leaf node
		val, ok := tags[f.Key]
		switch f.Compare {
		case FilterCompareEQ:
			return ok && val == f.Value, nil
		case FilterCompareNotEQ:
			return !ok || val != f.Value, nil
		case FilterCompareIn:
			return slices.Contains(f.ValueSet, val), nil
		case FilterCompareNotIn:
			return !slices.Contains(f.ValueSet, val), nil
		case FilterCompareExists:
			return ok, nil
		case FilterCompareNotExists:
			return !ok, nil
		default:
			return false, fmt.Errorf("invalid Compare value: %s", f.Compare)
		}
	case FilterOpAnd:
		for _, c := range f.Children {
			match, err := c.Match(tags)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil
			}
		}
		return true, nil
	case FilterOpOr:
		for _, c := range f.Children {
			match, err := c.Match(tags)
			if err != nil {
				return false, err
			}
			if match {
				return true, nil
			}
		}
		return false, nil
	case FilterOpNot:
		if len(f.Children) != 1 {
			return false, errors.New("NOT must have exactly one child")
		}
		match, err := f.Children[0].Match(tags)
		if err != nil {
			return false, err
		}
		return !match, nil
	default:
		return false, fmt.Errorf("invalid Op value: %s", f.Op)
	}
}
