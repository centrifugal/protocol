package protocol

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
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
	FilterComparePrefix    = "prefix"
	FilterCompareSuffix    = "suffix"
	FilterCompareContains  = "contains"
	FilterCompareIntGT     = "igt"
	FilterCompareIntGTE    = "igte"
	FilterCompareIntLT     = "ilt"
	FilterCompareIntLTE    = "ilte"
	FilterCompareFloatGT   = "fgt"
	FilterCompareFloatGTE  = "fgte"
	FilterCompareFloatLT   = "flt"
	FilterCompareFloatLTE  = "flte"
)

// FilterMatch evaluates the filter against a set of tags.
func FilterMatch(f *FilterNode, tags map[string]string) (bool, error) {
	switch f.Op {
	case FilterOpLeaf:
		val, ok := tags[f.Key]
		switch f.Compare {
		// string equality
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

		// integer numeric comparisons
		case FilterCompareIntGT, FilterCompareIntGTE, FilterCompareIntLT, FilterCompareIntLTE:
			if !ok {
				return false, nil
			}
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return false, nil
			}
			cmp, _ := strconv.ParseInt(f.Value, 10, 64)
			switch f.Compare {
			case FilterCompareIntGT:
				return v > cmp, nil
			case FilterCompareIntGTE:
				return v >= cmp, nil
			case FilterCompareIntLT:
				return v < cmp, nil
			case FilterCompareIntLTE:
				return v <= cmp, nil
			}

		// float numeric comparisons
		case FilterCompareFloatGT, FilterCompareFloatGTE, FilterCompareFloatLT, FilterCompareFloatLTE:
			if !ok {
				return false, nil
			}
			v, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return false, nil
			}
			cmp, _ := strconv.ParseFloat(f.Value, 64)
			switch f.Compare {
			case FilterCompareFloatGT:
				return v > cmp, nil
			case FilterCompareFloatGTE:
				return v >= cmp, nil
			case FilterCompareFloatLT:
				return v < cmp, nil
			case FilterCompareFloatLTE:
				return v <= cmp, nil
			}

		// string pattern comparisons
		case FilterComparePrefix:
			return ok && strings.HasPrefix(val, f.Value), nil
		case FilterCompareSuffix:
			return ok && strings.HasSuffix(val, f.Value), nil
		case FilterCompareContains:
			return ok && strings.Contains(val, f.Value), nil

		default:
			return false, fmt.Errorf("invalid Compare value: %s", f.Compare)
		}

	case FilterOpAnd:
		for _, c := range f.Children {
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
		for _, c := range f.Children {
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
		if len(f.Children) != 1 {
			return false, errors.New("NOT must have exactly one child")
		}
		match, err := FilterMatch(f.Children[0], tags)
		if err != nil {
			return false, err
		}
		return !match, nil

	default:
		return false, fmt.Errorf("invalid Op value: %s", f.Op)
	}
	// Fallback return (should never be reached if all cases handled).
	return false, fmt.Errorf("unhandled FilterNode configuration")
}
