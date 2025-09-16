package protocol

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/quagmt/udecimal"
)

// Node operations.
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
	FilterComparePrefix    = "starts"
	FilterCompareSuffix    = "ends"
	FilterCompareContains  = "contains"
	FilterCompareGT        = "gt"
	FilterCompareGTE       = "gte"
	FilterCompareLT        = "lt"
	FilterCompareLTE       = "lte"
)

func FilterMatch(f *FilterNode, tags map[string]string) (bool, error) {
	switch f.Op {
	case FilterOpLeaf:
		val, ok := tags[f.Key]
		switch f.Cmp {
		case FilterCompareEQ:
			return ok && val == f.Val, nil
		case FilterCompareNotEQ:
			return !ok || val != f.Val, nil
		case FilterCompareIn:
			return slices.Contains(f.Vals, val), nil
		case FilterCompareNotIn:
			return !slices.Contains(f.Vals, val), nil
		case FilterCompareExists:
			return ok, nil
		case FilterCompareNotExists:
			return !ok, nil
		case FilterComparePrefix:
			return ok && strings.HasPrefix(val, f.Val), nil
		case FilterCompareSuffix:
			return ok && strings.HasSuffix(val, f.Val), nil
		case FilterCompareContains:
			return ok && strings.Contains(val, f.Val), nil

		// numeric comparisons unified
		case FilterCompareGT, FilterCompareGTE, FilterCompareLT, FilterCompareLTE:
			if !ok {
				return false, nil
			}
			v, err := udecimal.Parse(val)
			if err != nil {
				return false, nil
			}
			cmp, err := udecimal.Parse(f.Val)
			if err != nil {
				return false, nil
			}
			switch f.Cmp {
			case FilterCompareGT:
				return v.Cmp(cmp) > 0, nil
			case FilterCompareGTE:
				return v.Cmp(cmp) >= 0, nil
			case FilterCompareLT:
				return v.Cmp(cmp) < 0, nil
			case FilterCompareLTE:
				return v.Cmp(cmp) <= 0, nil
			case FilterCompareEQ:
				return v.Cmp(cmp) == 0, nil
			}
		default:
			return false, fmt.Errorf("invalid Compare value: %s", f.Cmp)
		}

	case FilterOpAnd:
		for _, c := range f.Nodes {
			match, err := FilterMatch(c, tags)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil
			}
		}
		return true, nil

	case FilterOpOr:
		for _, c := range f.Nodes {
			match, err := FilterMatch(c, tags)
			if err != nil {
				return false, err
			}
			if match {
				return true, nil
			}
		}
		return false, nil

	case FilterOpNot:
		if len(f.Nodes) != 1 {
			return false, errors.New("NOT must have exactly one child")
		}
		match, err := FilterMatch(f.Nodes[0], tags)
		if err != nil {
			return false, err
		}
		return !match, nil
	default:
	}
	return false, fmt.Errorf("invalid filter op: %s", f.Op)
}
