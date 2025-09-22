package protocol

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/quagmt/udecimal"
)

// ⚠️ Challenge
// Centrifugo’s philosophy is to keep message routing simple and fast — filters might add overhead especially
// with many subscribers in channel (since in hot broadcast path). We must be careful with filter design
// to avoid security confusion also – because permissions will be still on channel level, and filter is just
// an additional layer of filtering for bandwidth/client-side processing optimizations.
//
// Thus Filter design decisions:
// - Must be zero allocation to evaluate, because it is on the hot path with many subscribers.
// - Must be easy to serialize/deserialize to/from Protobuf and fully JSON compatible.
// - Must be programmatically constructible.
// - Must be simple. It's a custom implementation, and we want to avoid too much complexity which can limit the usage.
// - Must be secure and not be Turing complete. Only filter based on what client can see in the Publication.
// - Server-side filter may be done separately and must not be controlled by clients.

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

// FilterValidate ensures the filter tree is well-formed and consistent.
// Call this at subscription time.
func FilterValidate(f *FilterNode) error {
	switch f.Op {
	case FilterOpLeaf:
		// Leaf must have a comparison operator.
		if f.Cmp == "" {
			return errors.New("leaf node must have cmp set")
		}

		switch f.Cmp {
		case FilterCompareEQ, FilterCompareNotEQ,
			FilterComparePrefix, FilterCompareSuffix, FilterCompareContains,
			FilterCompareGT, FilterCompareGTE, FilterCompareLT, FilterCompareLTE:
			if f.Val == "" {
				return fmt.Errorf("%s comparison requires Val", f.Cmp)
			}
			if len(f.Vals) > 0 {
				return fmt.Errorf("%s comparison must not use Vals", f.Cmp)
			}

		case FilterCompareIn, FilterCompareNotIn:
			if len(f.Vals) == 0 {
				return fmt.Errorf("%s comparison requires non-empty Vals", f.Cmp)
			}
			if f.Val != "" {
				return fmt.Errorf("%s comparison must not use Val", f.Cmp)
			}

		case FilterCompareExists, FilterCompareNotExists:
			if f.Val != "" || len(f.Vals) > 0 {
				return fmt.Errorf("%s comparison must not use Val or Vals", f.Cmp)
			}

		default:
			return fmt.Errorf("unknown comparison operator: %s", f.Cmp)
		}

		// All leafs must have a key except exists/nex
		if f.Key == "" &&
			f.Cmp != FilterCompareExists &&
			f.Cmp != FilterCompareNotExists {
			return errors.New("leaf node requires key")
		}

	case FilterOpAnd, FilterOpOr:
		if len(f.Nodes) == 0 {
			return fmt.Errorf("%s node must have at least one child", f.Op)
		}
		for _, c := range f.Nodes {
			if err := FilterValidate(c); err != nil {
				return err
			}
		}

	case FilterOpNot:
		if len(f.Nodes) != 1 {
			return errors.New("not node must have exactly one child")
		}
		return FilterValidate(f.Nodes[0])

	default:
		return fmt.Errorf("invalid op: %s", f.Op)
	}
	return nil
}

func FilterHash(f *FilterNode) [32]byte {
	bb := getByteBuffer(f.SizeVT())
	defer putByteBuffer(bb)
	n, _ := f.MarshalToVT(bb.B)    // get canonical hash.
	return sha256.Sum256(bb.B[:n]) // SHA-256 hash.
}
